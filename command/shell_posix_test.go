//go:build !windows
// +build !windows

package command

import (
	"strings"
	"testing"
)

func TestShellAsUser(t *testing.T) {
	cmd := NewSh(`whoami`).AsUser("nobody")
	err := cmd.Run()
	if !strings.Contains(err.Error(), "operation not permitted") {
		t.Fatal("AsUser failed", err)
	}
}
