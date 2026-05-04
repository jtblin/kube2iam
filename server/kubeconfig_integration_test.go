//go:build integration

package server

import (
	"testing"
)

func TestRunKubeconfigError(t *testing.T) {
	s := NewServer()
	// Pass a non-existent kubeconfig
	err := s.Run("/tmp/non-existent-kubeconfig", "", "", "node", false)
	if err != nil {
		// This is expected as the file doesn't exist
		return
	}
	t.Fatal("expected error when running with non-existent kubeconfig, got nil")
}
