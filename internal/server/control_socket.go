package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

func ListenControl(path string) (net.Listener, error) {
	if path == "" {
		return nil, fmt.Errorf("control socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		listener.Close()
		os.Remove(path)
		return nil, err
	}
	return listener, nil
}

func DialControl(ctx context.Context, path string) (net.Conn, error) {
	if path == "" {
		return nil, fmt.Errorf("control socket path is required")
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", path)
}

func ServeControlSocket(ctx context.Context, listener net.Listener, supervisor *Supervisor) error {
	defer listener.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go func() { _ = ServeControl(ctx, conn, supervisor) }()
	}
}
