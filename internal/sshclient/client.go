package sshclient

import (
	"fmt"
	"strings"
)

type Config struct {
	Host string
	Port int
	User string
	KeyPath string
	Password string
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" { return fmt.Errorf("ssh host is required") }
	if strings.TrimSpace(c.User) == "" { return fmt.Errorf("ssh user is required") }
	if c.KeyPath == "" && c.Password == "" { return fmt.Errorf("ssh key or password is required") }
	return nil
}
