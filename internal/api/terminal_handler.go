package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"github.com/azzliang6/opsup/internal/auth"
	"github.com/azzliang6/opsup/internal/crypto"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/azzliang6/opsup/internal/sshclient"
)

const (
	msgTypeStdin    = 0x00
	msgTypeResize   = 0x01
	msgTypeStdout   = 0x00
	msgTypeStderr   = 0x01
	msgTypeError    = 0x02
	msgTypeStatus   = 0x03
	msgTypeConnectd = 0x04
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// wsWriter serializes all WebSocket writes through a channel.
type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsWriter) write(msgType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(msgType, data)
}

func TerminalHandler(cfg AppConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, _ := strconv.ParseInt(c.Param("serverId"), 10, 64)
		log.Printf("[terminal] connection request for server %d from %s", serverID, c.ClientIP())

		// 1. Validate JWT
		tokenStr := c.Query("token")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := auth.ValidateToken(tokenStr, cfg.JWTSecret)
		if err != nil {
			log.Printf("[terminal] invalid token: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		log.Printf("[terminal] user %s authenticated for server %d", claims.Username, serverID)

		// 2. Lookup server
		server, err := models.GetServer(getDB(c), serverID)
		if err != nil || server == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		log.Printf("[terminal] server %s (%s:%d)", server.Name, server.Host, server.Port)

		// 3. Decrypt credentials
		privateKey, _ := crypto.Decrypt(server.PrivateKey, cfg.EncryptionKey)
		var passwordStr string
		if server.Password != "" {
			pwBytes, err := crypto.Decrypt(server.Password, cfg.EncryptionKey)
			if err == nil {
				passwordStr = string(pwBytes)
			}
		}

		// 4. WebSocket upgrade
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[terminal] ws upgrade failed: %v", err)
			return
		}
		defer ws.Close()

		w := &wsWriter{conn: ws}

		// Set read deadline and pong handler for keepalive
		ws.SetReadDeadline(time.Time{}) // no deadline, we use ping/pong
		ws.SetPongHandler(func(string) error {
			ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 5. SSH connect (direct or via jump host)
		var client *ssh.Client
		if server.JumpServerID != nil && *server.JumpServerID != 0 {
			// Connect through jump host
			jumpServer, err := models.GetServer(getDB(c), *server.JumpServerID)
			if err != nil || jumpServer == nil {
				log.Printf("[terminal] jump server %d not found", *server.JumpServerID)
				sendTypedMsg(w, msgTypeError, []byte("jump server not found"))
				return
			}
			jumpKey, _ := crypto.Decrypt(jumpServer.PrivateKey, cfg.EncryptionKey)
			var jumpPassword string
			if jumpServer.Password != "" {
				jpBytes, err := crypto.Decrypt(jumpServer.Password, cfg.EncryptionKey)
				if err == nil {
					jumpPassword = string(jpBytes)
				}
			}
			log.Printf("[terminal] dialing SSH %s@%s:%d via jump %s@%s:%d ...",
				server.Username, server.Host, server.Port,
				jumpServer.Username, jumpServer.Host, jumpServer.Port)
			client, err = sshclient.DialViaJump(
				jumpServer.Host, jumpServer.Port, jumpServer.Username, jumpServer.AuthType, jumpKey, jumpPassword,
				server.Host, server.Port, server.Username, server.AuthType, privateKey, passwordStr,
			)
			if err != nil {
				log.Printf("[terminal] SSH dial via jump failed: %v", err)
				sendTypedMsg(w, msgTypeError, []byte(fmt.Sprintf("SSH connection via jump failed: %v", err)))
				return
			}
		} else {
			log.Printf("[terminal] dialing SSH %s@%s:%d ...", server.Username, server.Host, server.Port)
			var err error
			client, err = sshclient.Dial(server.Host, server.Port, server.Username, server.AuthType, privateKey, passwordStr)
			if err != nil {
				log.Printf("[terminal] SSH dial failed: %v", err)
				sendTypedMsg(w, msgTypeError, []byte(fmt.Sprintf("SSH connection failed: %v", err)))
				return
			}
		}
		defer client.Close()
		log.Printf("[terminal] SSH connected")

		sess, err := sshclient.NewSession(client, 80, 24)
		if err != nil {
			log.Printf("[terminal] session failed: %v", err)
			sendTypedMsg(w, msgTypeError, []byte(fmt.Sprintf("SSH session failed: %v", err)))
			return
		}
		defer sess.Close()
		log.Printf("[terminal] session started, sending connected")

		// 6. Send connected
		connectedMsg, _ := json.Marshal(map[string]interface{}{"host": server.Host, "cols": 80, "rows": 24})
		sendTypedMsg(w, msgTypeConnectd, connectedMsg)

		// Shared done signal
		done := make(chan struct{})
		var once sync.Once

		// 7. Pump: SSH stdout -> WebSocket (serialized via mutex)
		go func() {
			buf := make([]byte, 8192)
			for {
				n, err := sess.Stdout().Read(buf)
				if err != nil {
					log.Printf("[terminal] stdout ended: %v", err)
					once.Do(func() { close(done) })
					return
				}
				msg := make([]byte, 1+n)
				msg[0] = msgTypeStdout
				copy(msg[1:], buf[:n])
				if err := w.write(websocket.BinaryMessage, msg); err != nil {
					once.Do(func() { close(done) })
					return
				}
			}
		}()

		// 8. Pump: SSH stderr -> WebSocket
		go func() {
			buf := make([]byte, 8192)
			for {
				n, err := sess.Stderr().Read(buf)
				if err != nil {
					return
				}
				msg := make([]byte, 1+n)
				msg[0] = msgTypeStderr
				copy(msg[1:], buf[:n])
				w.write(websocket.BinaryMessage, msg)
			}
		}()

		// 9. Pump: WebSocket -> SSH stdin
		go func() {
			for {
				_, msg, err := ws.ReadMessage()
				if err != nil {
					log.Printf("[terminal] ws read ended: %v", err)
					once.Do(func() { close(done) })
					return
				}
				if len(msg) < 1 {
					continue
				}
				switch msg[0] {
				case msgTypeStdin:
					if len(msg) > 1 {
						sess.Write(msg[1:])
					}
				case msgTypeResize:
					var resize struct {
						Cols int `json:"cols"`
						Rows int `json:"rows"`
					}
					if err := json.Unmarshal(msg[1:], &resize); err == nil {
						sess.Resize(resize.Cols, resize.Rows)
					}
				}
			}
		}()

		// 10. Keepalive: ping + SSH keepalive
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		log.Printf("[terminal] session active, keepalive loop started")
		for {
			select {
			case <-done:
				log.Printf("[terminal] session ended")
				disconnectMsg, _ := json.Marshal(map[string]interface{}{"connected": false, "message": "session ended"})
				w.write(websocket.BinaryMessage, append([]byte{msgTypeStatus}, disconnectMsg...))
				return
			case <-ticker.C:
				// Send WebSocket ping
				if err := w.write(websocket.PingMessage, nil); err != nil {
					log.Printf("[terminal] ping failed: %v", err)
					return
				}
			}
		}
	}
}

func sendTypedMsg(w *wsWriter, msgType byte, payload []byte) {
	msg := make([]byte, 1+len(payload))
	msg[0] = msgType
	copy(msg[1:], payload)
	w.write(websocket.BinaryMessage, msg)
}
