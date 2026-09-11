package sshclient

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const SetupTimeout = 15 * time.Second

type Config struct {
	Host       string
	Port       int
	Username   string
	AuthType   string
	PrivateKey []byte
	Password   string
	HostKey    string
}

// Client owns the target and, when present, the jump transport. Close tears down
// the physical socket first so blocked SSH channel writes cannot delay cleanup.
type Client struct {
	*ssh.Client
	transport net.Conn
	jump      *Client
	done      chan struct{}
	once      sync.Once
}

func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.done)
		if c.jump != nil {
			_ = c.jump.Close()
		}
		err = c.transport.Close()
	})
	return err
}

func (c *Client) watch(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-c.done:
		}
	}()
}

func hostKeyCallback(pin string) ssh.HostKeyCallback {
	return func(hostname string, _ net.Addr, key ssh.PublicKey) error {
		observed := ssh.FingerprintSHA256(key)
		if pin == "" {
			return fmt.Errorf("host key not pinned for %s; observed %s; independently verify this fingerprint before saving it", hostname, observed)
		}
		if subtle.ConstantTimeCompare([]byte(pin), []byte(observed)) != 1 {
			return fmt.Errorf("host key mismatch for %s: expected %s, observed %s; independently verify the server identity", hostname, pin, observed)
		}
		return nil
	}
}

func makeConfig(cfg Config) (*ssh.ClientConfig, error) {
	var method ssh.AuthMethod
	switch cfg.AuthType {
	case "password":
		if cfg.Password == "" {
			return nil, fmt.Errorf("no password available")
		}
		method = ssh.Password(cfg.Password)
	case "key":
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		method = ssh.PublicKeys(signer)
	default:
		return nil, fmt.Errorf("unsupported authentication type %q", cfg.AuthType)
	}
	return &ssh.ClientConfig{User: cfg.Username, Auth: []ssh.AuthMethod{method}, HostKeyCallback: hostKeyCallback(cfg.HostKey)}, nil
}

func Dial(ctx context.Context, cfg Config) (*Client, error) {
	config, err := makeConfig(cfg)
	if err != nil {
		return nil, err
	}
	setup, cancel := context.WithTimeout(ctx, SetupTimeout)
	defer cancel()
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := (&net.Dialer{}).DialContext(setup, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	owner := &Client{transport: conn, done: make(chan struct{})}
	stop := context.AfterFunc(setup, func() { _ = owner.Close() })
	defer stop()
	deadline, _ := setup.Deadline()
	_ = conn.SetDeadline(deadline)
	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = owner.Close()
		if setup.Err() != nil {
			err = setup.Err()
		}
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	owner.Client = ssh.NewClient(ncc, chans, reqs)
	if !stop() || setup.Err() != nil {
		_ = owner.Close()
		return nil, setup.Err()
	}
	_ = conn.SetDeadline(time.Time{})
	owner.watch(ctx)
	return owner, nil
}

func DialViaJump(ctx context.Context, jump, target Config) (*Client, error) {
	targetConfig, err := makeConfig(target)
	if err != nil {
		return nil, err
	}
	jumpClient, err := Dial(ctx, jump)
	if err != nil {
		return nil, fmt.Errorf("jump host: %w", err)
	}
	setup, cancel := context.WithTimeout(ctx, SetupTimeout)
	defer cancel()
	// SSH forwarded connections do not implement deadlines. Closing the physical
	// jump socket bounds both channel-open and nested SSH handshake operations.
	stop := context.AfterFunc(setup, func() { _ = jumpClient.Close() })
	defer stop()
	addr := net.JoinHostPort(target.Host, strconv.Itoa(target.Port))
	conn, err := jumpClient.Dial("tcp", addr)
	if err != nil {
		_ = jumpClient.Close()
		if setup.Err() != nil {
			err = setup.Err()
		}
		return nil, fmt.Errorf("jump dial %s: %w", addr, err)
	}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, targetConfig)
	if err != nil {
		_ = jumpClient.Close()
		_ = conn.Close()
		if setup.Err() != nil {
			err = setup.Err()
		}
		return nil, fmt.Errorf("target handshake %s: %w", addr, err)
	}
	owner := &Client{Client: ssh.NewClient(ncc, chans, reqs), transport: conn, jump: jumpClient, done: make(chan struct{})}
	if !stop() || setup.Err() != nil {
		_ = owner.Close()
		return nil, setup.Err()
	}
	owner.watch(ctx)
	return owner, nil
}
