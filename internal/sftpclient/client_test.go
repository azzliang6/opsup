package sftpclient

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/azzliang6/opsup/internal/sshclient"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func sftpPeer(t *testing.T, mode string) (sshclient.Config, <-chan struct{}) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	n, _ := strconv.Atoi(port)
	done := make(chan struct{})
	go func() {
		defer close(done)
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		t.Cleanup(func() { _ = raw.Close() })
		config := &ssh.ServerConfig{NoClientAuth: true}
		config.AddHostKey(signer)
		conn, channels, requests, err := ssh.NewServerConn(raw, config)
		if err != nil {
			return
		}
		defer conn.Close()
		go ssh.DiscardRequests(requests)
		var sessions sync.WaitGroup
		defer sessions.Wait()
		for incoming := range channels {
			if incoming.ChannelType() != "session" {
				_ = incoming.Reject(ssh.UnknownChannelType, "unsupported")
				continue
			}
			ch, requests, err := incoming.Accept()
			if err != nil {
				continue
			}
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				defer ch.Close()
				for req := range requests {
					if req.Type != "subsystem" {
						_ = req.Reply(false, nil)
						continue
					}
					if mode == "stall" {
						continue
					}
					_ = req.Reply(true, nil)
					if mode == "stall-io" {
						_, _ = ch.Write([]byte{0, 0, 0, 5, 2, 0, 0, 0, 3})
						_, _ = io.Copy(io.Discard, ch)
						return
					}
					if mode == "close-error" {
						handlers := sftp.InMemHandler()
						handlers.FilePut = closeFailureWriter{handlers.FilePut}
						server := sftp.NewRequestServer(ch, handlers)
						_ = server.Serve()
						_ = server.Close()
						return
					}
					server, err := sftp.NewServer(ch)
					if err != nil {
						return
					}
					_ = server.Serve()
					_ = server.Close()
					return
				}
			}()
		}
	}()
	return sshclient.Config{Host: host, Port: n, Username: "test", AuthType: "password", Password: "test", HostKey: ssh.FingerprintSHA256(signer.PublicKey())}, done
}

func newTestClient(t *testing.T, modes ...string) *Client {
	t.Helper()
	mode := ""
	if len(modes) > 0 {
		mode = modes[0]
	}
	cfg, done := sftpPeer(t, mode)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	owner, err := sshclient.Dial(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("SFTP peer did not close")
		}
	})
	return client
}

type failedSource struct{ sent bool }

func (r *failedSource) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errors.New("source failed")
	}
	r.sent = true
	return copy(p, "partial replacement"), errors.New("source failed")
}

func TestUploadPreservesDestinationOnSourceFailure(t *testing.T) {
	client := newTestClient(t)
	dir := t.TempDir()
	dest := filepath.Join(dir, "existing")
	if err := os.WriteFile(dest, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(dest, &failedSource{}); err == nil {
		t.Fatal("expected failed upload")
	}
	content, err := os.ReadFile(dest)
	if err != nil || string(content) != "original" {
		t.Fatalf("destination changed: %q, %v", content, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", files, err)
	}
}

func TestUploadAtomicallyReplacesDestination(t *testing.T) {
	client := newTestClient(t)
	dir := t.TempDir()
	dest := filepath.Join(dir, "existing")
	if err := os.WriteFile(dest, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(dest, strings.NewReader("replacement")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(dest)
	if err != nil || string(content) != "replacement" {
		t.Fatalf("incorrect upload: %q, %v", content, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", files, err)
	}
}

func TestCancelledSubsystemSetup(t *testing.T) {
	cfg, closed := sftpPeer(t, "stall")
	owner, err := sshclient.Dial(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = NewClient(ctx, owner)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("subsystem setup leaked connection")
	}
}

var _ io.Reader = (*failedSource)(nil)
