package sftpclient

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/sftp"
	"github.com/azzliang6/opsup/internal/sshclient"
	"golang.org/x/crypto/ssh"
)

type Client struct {
	sshClient *ssh.Client
	sftpClient *sftp.Client
}

func NewSFTPClient(host string, port int, username string, authType string, privateKey []byte, password string) (*Client, error) {
	sshClient, err := sshclient.Dial(host, port, username, authType, privateKey, password)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("sftp init: %w", err)
	}

	return &Client{sshClient: sshClient, sftpClient: sftpClient}, nil
}

func NewSFTPClientViaJump(
	jumpHost string, jumpPort int, jumpUser string, jumpAuthType string, jumpKey []byte, jumpPassword string,
	targetHost string, targetPort int, targetUser string, targetAuthType string, targetKey []byte, targetPassword string,
) (*Client, error) {
	sshClient, err := sshclient.DialViaJump(
		jumpHost, jumpPort, jumpUser, jumpAuthType, jumpKey, jumpPassword,
		targetHost, targetPort, targetUser, targetAuthType, targetKey, targetPassword,
	)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("sftp init: %w", err)
	}

	return &Client{sshClient: sshClient, sftpClient: sftpClient}, nil
}

func (c *Client) ReadDir(path string) ([]os.FileInfo, error) {
	return c.sftpClient.ReadDir(path)
}

func (c *Client) Create(path string) (*sftp.File, error) {
	return c.sftpClient.Create(path)
}

func (c *Client) Open(path string) (*sftp.File, error) {
	return c.sftpClient.Open(path)
}

func (c *Client) MkdirAll(path string) error {
	return c.sftpClient.MkdirAll(path)
}

func (c *Client) Remove(path string) error {
	return c.sftpClient.Remove(path)
}

func (c *Client) RemoveDirectory(path string) error {
	return c.sftpClient.RemoveDirectory(path)
}

func (c *Client) Stat(path string) (os.FileInfo, error) {
	return c.sftpClient.Stat(path)
}

func (c *Client) Close() error {
	c.sftpClient.Close()
	c.sshClient.Close()
	return nil
}

var _ io.Closer = (*Client)(nil)
