package sftpclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	"github.com/azzliang6/opsup/internal/sshclient"
	"github.com/pkg/sftp"
)

type Client struct {
	sshClient  *sshclient.Client
	sftpClient *sftp.Client
	once       sync.Once
	stop       func() bool
}

// NewClient takes ownership of the SSH connection, including on failure.
func NewClient(ctx context.Context, owner *sshclient.Client) (*Client, error) {
	setup, cancel := context.WithTimeout(ctx, sshclient.SetupTimeout)
	defer cancel()
	stop := context.AfterFunc(setup, func() { _ = owner.Close() })
	defer stop()
	client, err := sftp.NewClient(owner.Client)
	if err != nil {
		_ = owner.Close()
		if setup.Err() != nil {
			err = setup.Err()
		}
		return nil, fmt.Errorf("sftp init: %w", err)
	}
	c := &Client{sshClient: owner, sftpClient: client, stop: context.AfterFunc(ctx, func() { _ = owner.Close() })}
	if !stop() || setup.Err() != nil {
		_ = c.Close()
		return nil, setup.Err()
	}
	return c, nil
}

func (c *Client) ReadDir(path string) ([]os.FileInfo, error) { return c.sftpClient.ReadDir(path) }
func (c *Client) Open(path string) (*sftp.File, error)       { return c.sftpClient.Open(path) }
func (c *Client) MkdirAll(path string) error                 { return c.sftpClient.MkdirAll(path) }
func (c *Client) Remove(path string) error                   { return c.sftpClient.Remove(path) }
func (c *Client) RemoveDirectory(path string) error          { return c.sftpClient.RemoveDirectory(path) }
func (c *Client) Stat(path string) (os.FileInfo, error)      { return c.sftpClient.Stat(path) }

// Upload writes a sibling temporary file and only publishes after a successful
// close. SFTP v3 rename must reject existing destinations; the POSIX extension
// permits atomic replacement. Never fall back after a failed POSIX rename.
func (c *Client) Upload(remotePath string, src io.Reader) (err error) {
	mode := os.FileMode(0600)
	var ownership *sftp.FileStat
	if existing, statErr := c.sftpClient.Lstat(remotePath); statErr == nil {
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("upload destination must be a regular file")
		}
		mode = existing.Mode().Perm()
		ownership, _ = existing.Sys().(*sftp.FileStat)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("check destination: %w", statErr)
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := path.Join(path.Dir(remotePath), ".opsup-upload-"+hex.EncodeToString(nonce[:]))
	dst, err := c.sftpClient.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = c.sftpClient.Remove(temp)
		}
	}()
	if err := dst.Chmod(0600); err != nil {
		_ = dst.Close()
		return fmt.Errorf("secure temporary file: %w", err)
	}
	_, copyErr := io.Copy(dst, src)
	if copyErr == nil && ownership != nil {
		copyErr = dst.Chown(int(ownership.UID), int(ownership.GID))
	}
	if copyErr == nil {
		copyErr = dst.Chmod(mode)
	}
	closeErr := dst.Close()
	if copyErr != nil {
		return fmt.Errorf("upload: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close upload: %w", closeErr)
	}
	if _, supported := c.sftpClient.HasExtension("posix-rename@openssh.com"); supported {
		err = c.sftpClient.PosixRename(temp, remotePath)
	} else {
		// Lstat also detects dangling symlinks; Rename supplies the race-safe
		// no-overwrite behavior required by SFTP v3 after this early check.
		if _, statErr := c.sftpClient.Lstat(remotePath); statErr == nil {
			return fmt.Errorf("atomic overwrite unsupported by this SFTP server")
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("check destination: %w", statErr)
		}
		err = c.sftpClient.Rename(temp, remotePath)
	}
	if err != nil {
		return fmt.Errorf("publish upload: %w", err)
	}
	published = true
	return nil
}

func (c *Client) Close() error {
	c.once.Do(func() {
		c.stop()
		_ = c.sshClient.Close()
		_ = c.sftpClient.Close()
	})
	return nil
}

var _ io.Closer = (*Client)(nil)
