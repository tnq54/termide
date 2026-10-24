//go:build linux

package server

import (
	"testing"
)

func TestAcquireDatabaseLockRejectsSecondOwner(t *testing.T) {
	path := t.TempDir() + "/termide.lock"
	first, err := AcquireDatabaseLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := AcquireDatabaseLock(path)
	if err == nil {
		second.Close()
		t.Fatal("second lock accepted")
	}
}
