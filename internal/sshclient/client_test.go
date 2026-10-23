package sshclient

import (
	"context"
	"testing"
)

func TestConfigRejectsMissingHost(t *testing.T) {
	if err := (Config{User: "ubuntu"}).Validate(); err == nil { t.Fatal("expected missing host error") }
}

func TestConfigRequiresKeyOrPassword(t *testing.T) {
	if err := (Config{Host: "vps", User: "ubuntu"}).Validate(); err == nil { t.Fatal("expected auth error") }
}

func TestClientRejectsInvalidConfigBeforeDial(t *testing.T) {
	client := NewClient(Config{User: "ubuntu"})
	if _, err := client.Run(context.Background(), "pwd"); err == nil { t.Fatal("expected validation error") }
}
