package sshclient

import "testing"

func TestConfigRejectsMissingHost(t *testing.T) {
	if err := (Config{User: "ubuntu"}).Validate(); err == nil { t.Fatal("expected missing host error") }
}

func TestConfigRequiresKeyOrPassword(t *testing.T) {
	if err := (Config{Host: "vps", User: "ubuntu"}).Validate(); err == nil { t.Fatal("expected auth error") }
}
