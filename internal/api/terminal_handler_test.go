package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/azzliang6/opsup/internal/sshclient"
	"github.com/gorilla/websocket"
)

func wsTestURL(server *httptest.Server) string { return "ws" + strings.TrimPrefix(server.URL, "http") }

func TestTerminalOriginPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer server.Close()
	for _, origin := range []string{server.URL, "", "https://attacker.invalid"} {
		t.Run(origin, func(t *testing.T) {
			headers := http.Header{}
			if origin != "" {
				headers.Set("Origin", origin)
			}
			conn, response, err := websocket.DefaultDialer.Dial(wsTestURL(server), headers)
			if origin == "https://attacker.invalid" {
				if err == nil {
					conn.Close()
					t.Fatal("cross-origin connection accepted")
				}
				if response == nil || response.StatusCode != http.StatusForbidden {
					t.Fatalf("unexpected rejection: %v", response)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				_ = conn.Close()
			}
			if response != nil {
				_ = response.Body.Close()
			}
		})
	}
}

func TestTerminalDisconnectCancelsSetup(t *testing.T) {
	started := make(chan struct{})
	ended := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		runTerminal(r.Context(), ws, "test", func(ctx context.Context) (*sshclient.Client, error) {
			close(started)
			<-ctx.Done()
			close(ended)
			return nil, ctx.Err()
		})
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial(wsTestURL(server), nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("dial did not start")
	}
	_ = conn.Close()
	select {
	case <-ended:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect did not cancel SSH setup")
	}
}

func TestTerminalMessageLimitAndQueueOverflow(t *testing.T) {
	for _, mode := range []string{"oversized", "queue-overflow"} {
		t.Run(mode, func(t *testing.T) {
			ended := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ws, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				runTerminal(r.Context(), ws, "test", func(ctx context.Context) (*sshclient.Client, error) {
					<-ctx.Done()
					close(ended)
					return nil, ctx.Err()
				})
			}))
			defer server.Close()
			conn, _, err := websocket.DefaultDialer.Dial(wsTestURL(server), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if mode == "oversized" {
				_ = conn.WriteMessage(websocket.BinaryMessage, make([]byte, maxTerminalMessage+1))
			} else {
				for i := 0; i < terminalInputQueue+1; i++ {
					_ = conn.WriteMessage(websocket.BinaryMessage, []byte{msgTypeStdin, 'x'})
				}
			}
			select {
			case <-ended:
			case <-time.After(2 * time.Second):
				t.Fatal("input was not bounded")
			}
		})
	}
}

type finalReader struct{ done bool }

func (r *finalReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, "final bytes"), io.EOF
}

func TestTerminalDeliversDataAlongsideEOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		_ = pumpTerminalOutput(&wsWriter{conn: ws}, &finalReader{}, msgTypeStdout)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial(wsTestURL(server), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "\x00final bytes" {
		t.Fatalf("lost final output: %q", data)
	}
}
