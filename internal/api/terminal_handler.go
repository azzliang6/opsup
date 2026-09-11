package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/azzliang6/opsup/internal/auth"
	"github.com/azzliang6/opsup/internal/sshclient"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	msgTypeStdin       = 0x00
	msgTypeResize      = 0x01
	msgTypeStdout      = 0x00
	msgTypeStderr      = 0x01
	msgTypeError       = 0x02
	msgTypeStatus      = 0x03
	msgTypeConnectd    = 0x04
	maxTerminalMessage = 64 * 1024
	terminalInputQueue = 32
	wsWriteTimeout     = 5 * time.Second
)

// A nil CheckOrigin uses Gorilla's same-origin policy, including port checks.
var upgrader = websocket.Upgrader{ReadBufferSize: 8192, WriteBufferSize: 8192}

type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsWriter) write(msgType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout)); err != nil {
		return err
	}
	return w.conn.WriteMessage(msgType, data)
}

func TerminalHandler(cfg AppConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, err := strconv.ParseInt(c.Param("serverId"), 10, 64)
		if err != nil || serverID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
			return
		}
		token := c.Query("token")
		if _, err := auth.ValidateToken(token, cfg.JWTSecret); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		db := getDB(c)
		server, err := loadConnectionServer(c.Request.Context(), db, serverID)
		if err != nil || server == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		runTerminal(c.Request.Context(), ws, server.Host, func(ctx context.Context) (*sshclient.Client, error) {
			return dialServer(ctx, db, server, cfg.EncryptionKey)
		})
	}
}

func readTerminalInput(ctx context.Context, ws *websocket.Conn, input chan<- []byte, cancel context.CancelFunc) {
	defer cancel()
	ws.SetReadLimit(maxTerminalMessage)
	_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	for {
		kind, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}
		if kind != websocket.BinaryMessage || len(msg) == 0 {
			continue
		}
		select {
		case input <- msg:
		case <-ctx.Done():
			return
		default:
			// Never stop reading behind a blocked SSH write: disconnect and reclaim
			// transports instead of accumulating unbounded input or missing WS closure.
			return
		}
	}
}

func runTerminal(parent context.Context, ws *websocket.Conn, host string, dial func(context.Context) (*sshclient.Client, error)) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer ws.Close()
	stop := context.AfterFunc(ctx, func() { _ = ws.Close() })
	defer stop()
	w := &wsWriter{conn: ws}
	input := make(chan []byte, terminalInputQueue)
	go readTerminalInput(ctx, ws, input, cancel)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.write(websocket.PingMessage, nil); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	client, err := dial(ctx)
	if err != nil {
		_ = sendTypedMsg(w, msgTypeError, []byte(fmt.Sprintf("SSH connection failed: %v", err)))
		return
	}
	defer client.Close()
	sess, err := sshclient.NewSession(ctx, client, 80, 24)
	if err != nil {
		_ = sendTypedMsg(w, msgTypeError, []byte(fmt.Sprintf("SSH session failed: %v", err)))
		return
	}
	defer sess.Close()
	connected, _ := json.Marshal(map[string]interface{}{"host": host, "cols": 80, "rows": 24})
	if err := sendTypedMsg(w, msgTypeConnectd, connected); err != nil {
		return
	}
	var output sync.WaitGroup
	output.Add(2)
	for _, stream := range []struct {
		reader io.Reader
		kind   byte
	}{{sess.Stdout(), msgTypeStdout}, {sess.Stderr(), msgTypeStderr}} {
		go func(reader io.Reader, kind byte) {
			defer output.Done()
			if err := pumpTerminalOutput(w, reader, kind); err != nil {
				cancel()
			}
		}(stream.reader, stream.kind)
	}
	go func() { output.Wait(); cancel() }()
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-input:
				var err error
				switch msg[0] {
				case msgTypeStdin:
					err = sess.Write(msg[1:])
				case msgTypeResize:
					var size struct {
						Cols int `json:"cols"`
						Rows int `json:"rows"`
					}
					if json.Unmarshal(msg[1:], &size) != nil {
						return
					}
					err = sess.Resize(size.Cols, size.Rows)
				default:
					return
				}
				if err != nil {
					return
				}
			}
		}
	}()
	<-ctx.Done()
	// Transport cleanup is independent of a final browser write. The socket close
	// is the disconnect notification, including when the browser stops reading.
	_ = client.Close()
}

func pumpTerminalOutput(w *wsWriter, reader io.Reader, kind byte) error {
	buf := make([]byte, 8192)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if writeErr := sendTypedMsg(w, kind, buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func sendTypedMsg(w *wsWriter, kind byte, payload []byte) error {
	msg := make([]byte, 1+len(payload))
	msg[0] = kind
	copy(msg[1:], payload)
	return w.write(websocket.BinaryMessage, msg)
}
