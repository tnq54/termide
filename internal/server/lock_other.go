//go:build !linux

package server

import "fmt"

func AcquireDatabaseLock(string) (interface{ Close() error }, error) {
	return nil, fmt.Errorf("database locking is unsupported on this platform")
}
