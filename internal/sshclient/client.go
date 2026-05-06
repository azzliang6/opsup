package sshclient

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestDial attempts to connect to the SSH server and returns an error if it fails.
func TestDial(host string, port int, username string, authType string, privateKey []byte, password string) error {
	config, err := makeConfig(username, authType, privateKey, password)
	if err != nil {
		return err
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		return err
	}
	client.Close()
	return nil
}

// TestDialViaJump tests connection through a jump host.
func TestDialViaJump(jumpHost string, jumpPort int, jumpUser string, jumpAuthType string, jumpKey []byte, jumpPassword string, targetHost string, targetPort int, targetUser string, targetAuthType string, targetKey []byte, targetPassword string) error {
	jumpClient, err := Dial(jumpHost, jumpPort, jumpUser, jumpAuthType, jumpKey, jumpPassword)
	if err != nil {
		return fmt.Errorf("jump host: %w", err)
	}
	defer jumpClient.Close()

	conn, err := dialThroughJump(jumpClient, targetHost, targetPort, targetUser, targetAuthType, targetKey, targetPassword)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// Dial establishes a direct SSH connection.
func Dial(host string, port int, username string, authType string, privateKey []byte, password string) (*ssh.Client, error) {
	config, err := makeConfig(username, authType, privateKey, password)
	if err != nil {
		return nil, err
	}
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s:%d: %w", host, port, err)
	}
	return conn, nil
}

// DialViaJump establishes an SSH connection through a jump host.
// Returns the final SSH client connected to the target.
func DialViaJump(jumpHost string, jumpPort int, jumpUser string, jumpAuthType string, jumpKey []byte, jumpPassword string, targetHost string, targetPort int, targetUser string, targetAuthType string, targetKey []byte, targetPassword string) (*ssh.Client, error) {
	jumpClient, err := Dial(jumpHost, jumpPort, jumpUser, jumpAuthType, jumpKey, jumpPassword)
	if err != nil {
		return nil, fmt.Errorf("jump host %s:%d: %w", jumpHost, jumpPort, err)
	}

	targetClient, err := dialThroughJump(jumpClient, targetHost, targetPort, targetUser, targetAuthType, targetKey, targetPassword)
	if err != nil {
		jumpClient.Close()
		return nil, fmt.Errorf("target %s:%d via jump: %w", targetHost, targetPort, err)
	}

	return targetClient, nil
}

// dialThroughJump dials the target through an existing jump client connection.
func dialThroughJump(jumpClient *ssh.Client, targetHost string, targetPort int, targetUser string, targetAuthType string, targetKey []byte, targetPassword string) (*ssh.Client, error) {
	targetConfig, err := makeConfig(targetUser, targetAuthType, targetKey, targetPassword)
	if err != nil {
		return nil, err
	}

	// Open a TCP connection to the target through the jump host
	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)
	conn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("jump dial to %s: %w", targetAddr, err)
	}

	// Upgrade the TCP connection to SSH
	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, targetConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake to %s: %w", targetAddr, err)
	}

	return ssh.NewClient(ncc, chans, reqs), nil
}

func makeConfig(username string, authType string, privateKey []byte, password string) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if authType == "password" && password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	} else if len(privateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		return nil, fmt.Errorf("no authentication method available")
	}

	return &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}

// Ensure net import is available
var _ net.Conn
