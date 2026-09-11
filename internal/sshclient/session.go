package sshclient

import (
	"context"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

const defaultTerm = "xterm-256color"

type Session struct {
	client  *Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	once    sync.Once
	stop    func() bool
}

func NewSession(ctx context.Context, client *Client, cols, rows int) (*Session, error) {
	setup, cancel := context.WithTimeout(ctx, SetupTimeout)
	defer cancel()
	stop := context.AfterFunc(setup, func() { _ = client.Close() })
	defer stop()
	sess, err := newSession(client, cols, rows)
	if err != nil {
		_ = client.Close()
		if setup.Err() != nil {
			return nil, setup.Err()
		}
		return nil, err
	}
	sess.stop = context.AfterFunc(ctx, func() { _ = client.Close() })
	if !stop() || setup.Err() != nil {
		sess.Close()
		return nil, setup.Err()
	}
	return sess, nil
}

func newSession(client *Client, cols, rows int) (*Session, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	fail := func(err error) (*Session, error) {
		_ = client.Close()
		_ = sess.Close()
		return nil, err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return fail(fmt.Errorf("stdin pipe: %w", err))
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return fail(fmt.Errorf("stdout pipe: %w", err))
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return fail(fmt.Errorf("stderr pipe: %w", err))
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	if err := sess.RequestPty(defaultTerm, rows, cols, ssh.TerminalModes{
		ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return fail(fmt.Errorf("request pty: %w", err))
	}
	if err := sess.Shell(); err != nil {
		return fail(fmt.Errorf("start shell: %w", err))
	}
	return &Session{client: client, session: sess, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

func (s *Session) Write(data []byte) error {
	_, err := s.stdin.Write(data)
	return err
}

func (s *Session) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 1000 || rows > 1000 {
		return fmt.Errorf("invalid terminal dimensions")
	}
	return s.session.WindowChange(rows, cols)
}

func (s *Session) Stdout() io.Reader { return s.stdout }
func (s *Session) Stderr() io.Reader { return s.stderr }

func (s *Session) Close() {
	s.once.Do(func() {
		s.stop()
		_ = s.client.Close()
		_ = s.session.Close()
	})
}
