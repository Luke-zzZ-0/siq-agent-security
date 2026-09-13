package skillimport

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// Local-only fixture: a hostname resolving exclusively to loopback must be
// rejected before invoking git. The fake CLI performs no network requests.
func TestReviewerGitFetchRejectsPrivateDNSBeforeGit(t *testing.T) {
	ips, err := net.LookupIP("localhost.localdomain")
	if err != nil || len(ips) == 0 {
		t.Skip("local fixture hostname unavailable")
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			t.Skip("fixture hostname is not exclusively loopback")
		}
	}
	root := t.TempDir()
	marker := filepath.Join(root, "invoked")
	fake := filepath.Join(root, "git")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n: > '"+marker+"'\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root)
	_, err = fetchGitCLI(context.Background(), "https://localhost.localdomain/fixture.git", "", filepath.Join(root, "repo"))
	if _, e := os.Stat(marker); e == nil {
		t.Error("private DNS destination reached git CLI without being blocked")
	}
	if !errors.Is(err, ErrURLBlocked) {
		t.Errorf("want ErrURLBlocked, got %v", err)
	}
}
