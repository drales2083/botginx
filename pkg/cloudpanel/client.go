// Package cloudpanel provides an SSH client for interacting with CloudPanel hosting control panel.
// It wraps CloudPanel CLI commands (clpctl) and executes them via SSH.
package cloudpanel

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	// ConnectTimeout is the timeout for establishing SSH connections
	ConnectTimeout = 10 * time.Second
	// CommandTimeout is the timeout for executing commands
	CommandTimeout = 60 * time.Second
)

// Client wraps an SSH connection to a CloudPanel server
type Client struct {
	conn   *ssh.Client
	config *ssh.ClientConfig
	host   string
}

// NewClient creates a new CloudPanel client (does not connect until needed).
// host: server IP or hostname
// port: SSH port (typically 22)
// username: SSH user (typically root for CloudPanel)
// password: SSH password
func NewClient(host string, port int, username, password string) *Client {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         ConnectTimeout,
	}
	return &Client{
		config: config,
		host:   fmt.Sprintf("%s:%d", host, port),
	}
}

// Connect establishes the SSH connection to the CloudPanel server
func (c *Client) Connect() error {
	conn, err := ssh.Dial("tcp", c.host, c.config)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", c.host, err)
	}
	c.conn = conn
	return nil
}

// Close closes the SSH connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// IsConnected returns true if the client has an active connection
func (c *Client) IsConnected() bool {
	return c.conn != nil
}

// Execute runs a command on the CloudPanel server and returns the output.
// It auto-connects if not already connected.
func (c *Client) Execute(cmd string) (string, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return "", err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return strings.TrimSpace(string(output)), err
}

// shellEscape escapes a string for safe use in shell commands.
// Uses single quotes and escapes embedded single quotes.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
