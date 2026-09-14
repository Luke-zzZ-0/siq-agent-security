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
// rejected by URL policy before any transport, DNS lookup, or process
// invocation. The fake git CLI on PATH performs no network requests and only
// records that it was never run.
func TestReviewerGitFetchRejectsPrivateDNSBeforeTransport(t *testing.T) {
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
	_, err = fetchHostedGit(context.Background(), "https://localhost.localdomain/fixture.git", "", filepath.Join(root, "repo"))
	if _, e := os.Stat(marker); e == nil {
		t.Error("private DNS destination reached a transport without being blocked")
	}
	if !errors.Is(err, ErrURLBlocked) {
		t.Errorf("want ErrURLBlocked, got %v", err)
	}
}
