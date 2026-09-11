package sshclient

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type testPeer struct {
	config Config
	closed chan struct{}
}

func startPeer(t *testing.T, mode string) testPeer {
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
	portNum, _ := strconv.Atoi(port)
	peer := testPeer{config: Config{Host: host, Port: portNum, Username: "test", AuthType: "password", Password: "test", HostKey: ssh.FingerprintSHA256(signer.PublicKey())}, closed: make(chan struct{})}
	go func() {
		defer close(peer.closed)
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
		for incoming := range channels {
			switch incoming.ChannelType() {
			case "direct-tcpip":
				if mode == "stall-forward" {
					continue
				}
				var dest struct {
					Host       string
					Port       uint32
					Origin     string
					OriginPort uint32
				}
				if ssh.Unmarshal(incoming.ExtraData(), &dest) != nil {
					_ = incoming.Reject(ssh.ConnectionFailed, "bad target")
					continue
				}
				target, err := net.Dial("tcp", net.JoinHostPort(dest.Host, strconv.Itoa(int(dest.Port))))
				if err != nil {
					_ = incoming.Reject(ssh.ConnectionFailed, "unreachable")
					continue
				}
				ch, requests, err := incoming.Accept()
				if err != nil {
					_ = target.Close()
					continue
				}
				go ssh.DiscardRequests(requests)
				go func() { _, _ = io.Copy(target, ch); _ = target.Close(); _ = ch.Close() }()
				go func() { _, _ = io.Copy(ch, target); _ = ch.Close(); _ = target.Close() }()
			case "session":
				if mode == "stall-session" {
					continue
				}
				ch, requests, err := incoming.Accept()
				if err != nil {
					continue
				}
				go func() {
					defer ch.Close()
					for req := range requests {
						if mode == "stall-pty" {
							continue
						}
						_ = req.Reply(true, nil)
					}
				}()
			default:
				_ = incoming.Reject(ssh.UnknownChannelType, "unsupported")
			}
		}
	}()
	return peer
}

func awaitClosed(t *testing.T, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("transport did not close")
	}
}

func TestHostKeyPins(t *testing.T) {
	for _, name := range []string{"missing", "mismatch", "match"} {
		t.Run(name, func(t *testing.T) {
			peer := startPeer(t, "")
			cfg := peer.config
			observed := cfg.HostKey
			if name == "missing" {
				cfg.HostKey = ""
			}
			if name == "mismatch" {
				cfg.HostKey = "SHA256:wrong"
			}
			client, err := Dial(context.Background(), cfg)
			if name == "match" {
				if err != nil {
					t.Fatal(err)
				}
				_ = client.Close()
			} else if err == nil || !strings.Contains(err.Error(), observed) {
				if client != nil {
					_ = client.Close()
				}
				t.Fatalf("expected rejection containing observed fingerprint, got %v", err)
			}
			awaitClosed(t, peer.closed)
		})
	}
}

func TestJumpOwnsBothTransports(t *testing.T) {
	target := startPeer(t, "")
	jump := startPeer(t, "")
	client, err := DialViaJump(context.Background(), jump.config, target.config)
	if err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	_ = client.Close()
	awaitClosed(t, target.closed)
	awaitClosed(t, jump.closed)
}

func TestJumpVerifiesTargetAndClosesOnFailure(t *testing.T) {
	target := startPeer(t, "")
	jump := startPeer(t, "")
	cfg := target.config
	cfg.HostKey = ""
	client, err := DialViaJump(context.Background(), jump.config, cfg)
	if client != nil {
		_ = client.Close()
	}
	if err == nil || !strings.Contains(err.Error(), target.config.HostKey) {
		t.Fatalf("unexpected result: %v", err)
	}
	awaitClosed(t, target.closed)
	awaitClosed(t, jump.closed)
}

func TestJumpVerifiesJumpBeforeForwarding(t *testing.T) {
	jump := startPeer(t, "")
	cfg := jump.config
	cfg.HostKey = ""
	_, err := DialViaJump(context.Background(), cfg, jump.config)
	if err == nil || !strings.Contains(err.Error(), jump.config.HostKey) {
		t.Fatalf("unexpected result: %v", err)
	}
	awaitClosed(t, jump.closed)
}

func TestCancelledStalledHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		_, _ = io.Copy(io.Discard, conn)
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	n, _ := strconv.Atoi(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := Dial(ctx, Config{Host: host, Port: n, Username: "test", AuthType: "password", Password: "test"})
		result <- err
	}()
	awaitClosed(t, accepted)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handshake remained blocked")
	}
	awaitClosed(t, closed)
}

func TestCancelledStalledJumpChannel(t *testing.T) {
	jump := startPeer(t, "stall-forward")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := DialViaJump(ctx, jump.config, jump.config)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
	awaitClosed(t, jump.closed)
}

func TestCancelledSessionSetup(t *testing.T) {
	for _, mode := range []string{"stall-session", "stall-pty"} {
		t.Run(mode, func(t *testing.T) {
			peer := startPeer(t, mode)
			client, err := Dial(context.Background(), peer.config)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, err = NewSession(ctx, client, 80, 24)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("want deadline exceeded, got %v", err)
			}
			awaitClosed(t, peer.closed)
		})
	}
}

func TestSessionCloseUnblocksReadAndWrite(t *testing.T) {
	peer := startPeer(t, "")
	client, err := Dial(context.Background(), peer.config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := NewSession(context.Background(), client, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	writeDone := make(chan struct{})
	readDone := make(chan struct{})
	go func() { _ = session.Write(bytes.Repeat([]byte("x"), 8*1024*1024)); close(writeDone) }()
	go func() { _, _ = session.Stdout().Read(make([]byte, 1)); close(readDone) }()
	select {
	case <-writeDone:
		t.Fatal("test requires a blocked write")
	case <-time.After(100 * time.Millisecond):
	}
	closeDone := make(chan struct{})
	go func() { session.Close(); close(closeDone) }()
	awaitClosed(t, closeDone)
	awaitClosed(t, writeDone)
	awaitClosed(t, readDone)
	awaitClosed(t, peer.closed)
}
