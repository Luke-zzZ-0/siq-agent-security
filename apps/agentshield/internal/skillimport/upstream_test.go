package skillimport

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// An upstream check is read-only: it re-fetches the recorded source and
// reports what it saw, without pinning, publishing, or modifying the signed
// record.
func TestUpstreamCheckGitSeesAdvancedHeadWithoutTouchingRecord(t *testing.T) {
	fixtureURL, commit, fixtureRoot := gitFixtureRepo(t)
	s, req := gitTestStore(t, fixtureURL)
	record, _, reused, err := s.CreateGit(context.Background(), *req)
	if err != nil || reused {
		t.Fatal(err, reused)
	}
	if record.Git.CommitSHA != commit {
		t.Fatal(record.Git.CommitSHA)
	}
	same, err := s.CheckUpstream(context.Background(), record.ImportID, "")
	if err != nil {
		t.Fatal(err)
	}
	if same.SourceKind != "git" || same.CommitSHA != commit || same.URL != req.URL || !same.ExcludedGitMetadata || len(same.Files) == 0 {
		t.Fatal(same)
	}
	// Advance the fixture repository: the check must observe the new HEAD
	// without asserting the pinned commit.
	path := filepath.Join(fixtureRoot, "repo", "docs", "new.md")
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("added upstream\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = filepath.Join(fixtureRoot, "repo")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	run("add", "-A")
	run("-c", "user.email=fixture@example.com", "-c", "user.name=fixture", "commit", "-q", "-m", "advance")
	advanced := strings.TrimSpace(run("rev-parse", "HEAD"))

	next, err := s.CheckUpstream(context.Background(), record.ImportID, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.CommitSHA != advanced || next.CommitSHA == commit {
		t.Fatal(next.CommitSHA)
	}
	var newFile bool
	for _, file := range next.Files {
		if file.Path == "docs/new.md" {
			newFile = true
		}
		if strings.Contains(file.Path, ".git") {
			t.Fatal("git metadata leaked into snapshot", file.Path)
		}
	}
	if !newFile {
		t.Fatal("advanced content missing", next.Files)
	}
	// The signed record is untouched: pinned commit and signature unchanged.
	after, err := s.ReadRecord(context.Background(), record.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Git.CommitSHA != commit || after.Signature != record.Signature || after.ArtifactDigest != record.ArtifactDigest {
		t.Fatal("upstream check modified the record", after.Git)
	}
	// Staging artifacts are cleaned up.
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "upstream-") {
			t.Fatal("upstream staging retained", entry.Name())
		}
	}
}

func TestUpstreamCheckZipBindsCallerURLAndRejectsOthers(t *testing.T) {
	s, _ := storeFixture(t)
	req := remoteRequest()
	raw := remoteArchive(t)
	req.ExpectedSHA256 = sum(raw)
	s.download = func(context.Context, string) (downloadedArchive, error) { return downloadedArchive{raw, req.URL}, nil }
	record, _, reused, err := s.CreateRemote(context.Background(), req)
	if err != nil || reused {
		t.Fatal(err, reused)
	}
	same, err := s.CheckUpstream(context.Background(), record.ImportID, req.URL)
	if err != nil {
		t.Fatal(err)
	}
	if same.SourceKind != "https_zip" || same.ArchiveSHA256 != sum(raw) || same.ArchiveBytes != int64(len(raw)) || same.SubDir != req.ArchivePath || len(same.Files) != len(record.Files) {
		t.Fatal(same)
	}
	// The record does not retain the URL; only a caller URL that reproduces
	// the signed source locator is accepted.
	if _, err = s.CheckUpstream(context.Background(), record.ImportID, req.URL+"&version=2"); !errors.Is(err, ErrChanged) {
		t.Fatal("unbound URL accepted", err)
	}
	if _, err = s.CheckUpstream(context.Background(), record.ImportID, "http://download.example.com/repo.zip"); !errors.Is(err, ErrURLBlocked) {
		t.Fatal("non-https URL accepted", err)
	}
	// A changed archive is reported with the new digest; the record stays put.
	changed := zipBytes(t, []zipMember{{req.ArchivePath + "/SKILL.md", []byte("---\nname: import-fixture\ndescription: Changed upstream.\n---\nChanged.\n"), 0600}})
	s.download = func(context.Context, string) (downloadedArchive, error) {
		return downloadedArchive{changed, req.URL}, nil
	}
	next, err := s.CheckUpstream(context.Background(), record.ImportID, req.URL)
	if err != nil {
		t.Fatal(err)
	}
	if next.ArchiveSHA256 != sum(changed) || next.ArchiveSHA256 == same.ArchiveSHA256 || len(next.Files) != 1 {
		t.Fatal(next)
	}
	after, err := s.ReadRecord(context.Background(), record.ImportID)
	if err != nil || after.Remote.ArchiveSHA256 != sum(raw) || after.Signature != record.Signature {
		t.Fatal("upstream check modified the record", after, err)
	}
}

func TestUpstreamCheckRejectsLocalAndUnknown(t *testing.T) {
	s, local := storeFixture(t)
	record, _, _, err := s.Create(context.Background(), local)
	if err != nil {
		t.Fatal(err)
	}
	// local_dir/local_zip imports have no fetchable upstream.
	if _, err = s.CheckUpstream(context.Background(), record.ImportID, ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("local source accepted", err)
	}
	if _, err = s.CheckUpstream(context.Background(), "nope", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("bad id accepted", err)
	}
	if _, err = s.CheckUpstream(context.Background(), "si-"+strings.Repeat("9", 32), ""); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown id accepted", err)
	}
}

func TestUpstreamFailureAndCancellationCleanTemporaryFiles(t *testing.T) {
	s, _ := storeFixture(t)
	req := remoteRequest()
	raw := remoteArchive(t)
	s.download = func(context.Context, string) (downloadedArchive, error) { return downloadedArchive{raw, req.URL}, nil }
	rec, _, _, err := s.CreateRemote(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		s.download = func(context.Context, string) (downloadedArchive, error) {
			if cancelled {
				cancel()
				return downloadedArchive{}, ctx.Err()
			}
			return downloadedArchive{}, ErrDownloadFailed
		}
		_, err = s.CheckUpstream(ctx, rec.ImportID, req.URL)
		cancel()
		if err == nil {
			t.Fatal("failed fetch accepted")
		}
		entries, e := os.ReadDir(s.dir)
		if e != nil {
			t.Fatal(e)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "upstream-") {
				t.Fatal("temporary fetch retained", entry.Name())
			}
		}
		after, e := s.ReadRecord(context.Background(), rec.ImportID)
		if e != nil || after.Signature != rec.Signature {
			t.Fatal("record changed", e)
		}
	}
}

func TestProductionGitGateDoesNotInvokeTransport(t *testing.T) {
	for _, url := range []string{"https://git.example.com/skill.git", "https://8.8.8.8/skill.git", "https://localhost.localdomain/skill.git"} {
		dst := filepath.Join(t.TempDir(), "uncreated")
		if _, err := fetchGitCLI(context.Background(), url, "main", dst); !errors.Is(err, ErrGitTransportUnavailable) {
			t.Fatal(url, err)
		}
		if _, err := os.Lstat(dst); !os.IsNotExist(err) {
			t.Fatal("disabled transport created target", err)
		}
	}
}
