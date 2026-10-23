package sshclient

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Host string
	Port int
	User string
	KeyPath string
	Password string
	Timeout time.Duration
}

type Client struct { config Config }

func NewClient(config Config) *Client { return &Client{config: config} }

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" { return fmt.Errorf("ssh host is required") }
	if strings.TrimSpace(c.User) == "" { return fmt.Errorf("ssh user is required") }
	if c.KeyPath == "" && c.Password == "" { return fmt.Errorf("ssh key or password is required") }
	return nil
}

func (c *Client) Run(ctx context.Context, command string) ([]byte, error) {
	if err := c.config.Validate(); err != nil { return nil, err }
	if strings.TrimSpace(command) == "" { return nil, fmt.Errorf("ssh command is required") }
	client, err := c.connect(ctx)
	if err != nil { return nil, err }
	defer client.Close()
	session, err := client.NewSession()
	if err != nil { return nil, err }
	defer session.Close()
	return session.CombinedOutput(command)
}

func (c *Client) connect(ctx context.Context) (*ssh.Client, error) {
	auth, err := c.authMethod(); if err != nil { return nil, err }
	port := c.config.Port; if port == 0 { port = 22 }
	config := &ssh.ClientConfig{User: c.config.User, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	address := fmt.Sprintf("%s:%d", c.config.Host, port)
	timeout := c.config.Timeout; if timeout == 0 { timeout = 10 * time.Second }
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address); if err != nil { return nil, err }
	cc, chans, reqs, err := ssh.NewClientConn(conn, address, config); if err != nil { conn.Close(); return nil, err }
	return ssh.NewClient(cc, chans, reqs), nil
}

func (c *Client) authMethod() (ssh.AuthMethod, error) {
	if c.config.KeyPath != "" {
		data, err := os.ReadFile(c.config.KeyPath); if err != nil { return nil, err }
		signer, err := ssh.ParsePrivateKey(data); if err != nil { return nil, err }
		return ssh.PublicKeys(signer), nil
	}
	return ssh.Password(c.config.Password), nil
}
