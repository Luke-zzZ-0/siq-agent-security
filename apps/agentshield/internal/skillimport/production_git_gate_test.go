package skillimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHostedGitProductionGatePreservesDestination(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "not-created")
	_, err := productionGitFetch(context.Background(), "https://github.com/octocat/Hello-World", "main", dst)
	if !errors.Is(err, ErrGitTransportUnavailable) {
		t.Fatal("unverified production transport enabled", err)
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Fatal("gate wrote destination", err)
	}
}
