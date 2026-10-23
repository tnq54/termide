package client

import (
	"context"
	"io"
	"os/exec"
	"reflect"
	"testing"
)

func TestProcessDuplexAndClose(t *testing.T) {
	stream, err := StartProcess(exec.CommandContext(context.Background(), "cat"))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 6)
	if _, err := io.ReadFull(stream, data); err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" {
		t.Fatal(string(data))
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSSHCommandUsesKnownHostsAndNoPTY(t *testing.T) {
	cmd, err := SSHCommand(context.Background(), "dev-vps")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ssh", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "--", "dev-vps", "termide-server --stdio"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatal(cmd.Args)
	}
	if _, err := SSHCommand(context.Background(), "-oProxyCommand=bad"); err == nil {
		t.Fatal("option accepted as host")
	}
}
