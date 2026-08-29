package sshexec

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client wraps an SSH connection for running commands
type Client struct {
	conn *ssh.Client
}

// NewClient creates a new SSH client with password auth
func NewClient(host, port, user, password string) (*Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &Client{conn: conn}, nil
}

// Run executes a command and returns the output
func (c *Client) Run(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		// Include stderr in error if available
		if stderr.Len() > 0 {
			return stdout.String(), fmt.Errorf("%w: %s", err, stderr.String())
		}
		return stdout.String(), err
	}

	return stdout.String(), nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
