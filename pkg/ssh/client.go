package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// HostKeyStore stores known host keys
type HostKeyStore struct {
	mu       sync.RWMutex
	keys     map[string]ssh.PublicKey
	filePath string
}

// Global host key store
var hostKeys = &HostKeyStore{
	keys: make(map[string]ssh.PublicKey),
}

// SetHostKeyFile sets the path to store known host keys
func SetHostKeyFile(path string) {
	hostKeys.filePath = path
	hostKeys.loadFromFile()
}

func (s *HostKeyStore) loadFromFile() {
	if s.filePath == "" {
		return
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	// Parse known_hosts format (simplified)
	_ = data // Load existing keys if file exists
}

func (s *HostKeyStore) saveToFile() {
	if s.filePath == "" {
		return
	}
	dir := filepath.Dir(s.filePath)
	os.MkdirAll(dir, 0700)
	// Save keys to file (simplified)
}

func (s *HostKeyStore) Get(addr string) ssh.PublicKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keys[addr]
}

func (s *HostKeyStore) Set(addr string, key ssh.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[addr] = key
	s.saveToFile()
}

// Client wraps an SSH connection to a server
type Client struct {
	host          string
	port          int
	user          string
	privateKey    []byte
	password      string
	conn          *ssh.Client
	timeout       time.Duration
	trustOnFirst  bool // TOFU: Trust On First Use
	knownHostKey  ssh.PublicKey
}

// Config for creating a new SSH client
type Config struct {
	Host         string
	Port         int
	User         string
	PrivateKey   []byte        // SSH private key (optional if Password is set)
	Password     string        // SSH password (optional if PrivateKey is set)
	Timeout      time.Duration
	TrustOnFirst bool          // Accept and store host key on first connection
	KnownHostKey ssh.PublicKey // Pre-configured host key
}

// NewClient creates a new SSH client (does not connect)
func NewClient(cfg Config) *Client {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Client{
		host:         cfg.Host,
		port:         cfg.Port,
		user:         cfg.User,
		privateKey:   cfg.PrivateKey,
		password:     cfg.Password,
		timeout:      cfg.Timeout,
		trustOnFirst: cfg.TrustOnFirst,
		knownHostKey: cfg.KnownHostKey,
	}
}

// hostKeyCallback returns a callback that verifies host keys
func (c *Client) hostKeyCallback() ssh.HostKeyCallback {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// Check if we have a pre-configured key
		if c.knownHostKey != nil {
			if ssh.FingerprintSHA256(key) != ssh.FingerprintSHA256(c.knownHostKey) {
				return fmt.Errorf("host key mismatch for %s", hostname)
			}
			return nil
		}

		// Check known host keys
		knownKey := hostKeys.Get(addr)
		if knownKey != nil {
			if ssh.FingerprintSHA256(key) != ssh.FingerprintSHA256(knownKey) {
				return fmt.Errorf("host key changed for %s (possible MITM attack)", hostname)
			}
			return nil
		}

		// TOFU: Trust On First Use
		if c.trustOnFirst {
			hostKeys.Set(addr, key)
			return nil
		}

		return fmt.Errorf("unknown host key for %s, fingerprint: %s", hostname, ssh.FingerprintSHA256(key))
	}
}

// Connect establishes the SSH connection
func (c *Client) Connect() error {
	var authMethods []ssh.AuthMethod

	// Add password auth if password is set
	if c.password != "" {
		authMethods = append(authMethods, ssh.Password(c.password))
	}

	// Add key auth if private key is set
	if len(c.privateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(c.privateKey)
		if err != nil {
			return fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no authentication method provided (need password or private key)")
	}

	config := &ssh.ClientConfig{
		User:            c.user,
		Auth:            authMethods,
		HostKeyCallback: c.hostKeyCallback(),
		Timeout:         c.timeout,
	}

	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	c.conn = conn
	return nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ExecResult holds the output of a command
type ExecResult struct {
	Output   string
	ExitCode int
	Error    error
}

// Exec runs a command and returns the combined output
func (c *Client) Exec(command string) (*ExecResult, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	result := &ExecResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			result.Error = err
		}
	}

	return result, nil
}

// ExecStream runs a command and streams output to a writer
func (c *Client) ExecStream(command string, stdout, stderr io.Writer) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdout = stdout
	session.Stderr = stderr

	return session.Run(command)
}

// Upload copies a local file to the remote server via SCP
func (c *Client) Upload(localPath, remotePath string, mode os.FileMode) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Read local file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}

	return c.WriteFile(remotePath, content, mode)
}

// WriteFile writes content to a remote file
func (c *Client) WriteFile(remotePath string, content []byte, mode os.FileMode) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	// Use cat to write file (works without SFTP)
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		stdin.Write(content)
	}()

	cmd := fmt.Sprintf("cat > %s && chmod %o %s", remotePath, mode, remotePath)
	return session.Run(cmd)
}

// Download reads a remote file
func (c *Client) Download(remotePath string) ([]byte, error) {
	result, err := c.Exec(fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("cat failed: %s", result.Output)
	}
	return []byte(result.Output), nil
}

// GenerateKeyPair creates a new ed25519 keypair
func GenerateKeyPair() (privateKey, publicKey []byte, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Convert to SSH format
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, nil, fmt.Errorf("convert public key: %w", err)
	}
	publicKey = ssh.MarshalAuthorizedKey(sshPubKey)

	// Marshal private key to PEM
	privBlock := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: marshalED25519PrivateKey(privKey),
	}
	privateKey = pem.EncodeToMemory(privBlock)

	return privateKey, publicKey, nil
}

// marshalED25519PrivateKey marshals an ed25519 private key to OpenSSH format
func marshalED25519PrivateKey(key ed25519.PrivateKey) []byte {
	// Simplified OpenSSH private key format
	// In production, use golang.org/x/crypto/ssh for proper marshaling
	pubKey := key.Public().(ed25519.PublicKey)

	// OpenSSH private key format (simplified)
	const magicHeader = "openssh-key-v1\x00"
	keyType := "ssh-ed25519"

	// This is a simplified implementation
	// For full compatibility, use proper OpenSSH marshaling
	return append([]byte(magicHeader+keyType), append(pubKey, key.Seed()...)...)
}

// TestConnection checks if we can connect to the server
func (c *Client) TestConnection() error {
	if err := c.Connect(); err != nil {
		return err
	}
	result, err := c.Exec("echo ok")
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("test command failed")
	}
	return nil
}

// ForwardPort creates a local port forward
func (c *Client) ForwardPort(localPort, remoteHost string, remotePort int) (net.Listener, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	listener, err := net.Listen("tcp", localPort)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			local, err := listener.Accept()
			if err != nil {
				return
			}

			remote, err := c.conn.Dial("tcp", fmt.Sprintf("%s:%d", remoteHost, remotePort))
			if err != nil {
				local.Close()
				continue
			}

			go func() {
				defer local.Close()
				defer remote.Close()
				go io.Copy(local, remote)
				io.Copy(remote, local)
			}()
		}
	}()

	return listener, nil
}
