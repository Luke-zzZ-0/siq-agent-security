package skillinstall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/skillimport"
)

var (
	gitFixtureURL    = "https://git.example.com/org/skill.git"
	gitFixtureCommit = "c" + strings.Repeat("0", 39)
	zipFixtureURL    = "https://download.example.com/repo.zip"
	zipFixtureDigest = "d" + strings.Repeat("2", 63)
	locatorFixture   = "e" + strings.Repeat("3", 63)
)

// rewriteImportRecord rewrites the fixture import record on disk into an
// upstream-capable shape and re-signs it with the fixture's shared key,
// keeping the artifact binding so the installed Plan.Source still links.
func rewriteImportRecord(t *testing.T, f fixture, mutate func(map[string]any)) {
	t.Helper()
	path := filepath.Join(filepath.Dir(f.store.dir), "skill-imports", "records", f.importID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := canon.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := value.(map[string]any)
	if !ok {
		t.Fatal("record is not an object")
	}
	mutate(doc)
	delete(doc, "signature")
	signature, err := f.store.key.SignCanonical(doc)
	if err != nil {
		t.Fatal(err)
	}
	doc["signature"] = signature
	signed, err := canon.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, signed, 0600); err != nil {
		t.Fatal(err)
	}
}

func gitifyImportRecord(t *testing.T, f fixture) {
	t.Helper()
	rewriteImportRecord(t, f, func(doc map[string]any) {
		doc["schema_version"] = "local-skill-import/v2"
		doc["source_kind"] = "git"
		doc["git"] = map[string]any{"url": gitFixtureURL, "ref": "", "sub_dir": "", "expected_commit": "", "commit_sha": gitFixtureCommit}
		doc["excluded_git_metadata"] = true
		canonical, err := canon.Marshal(map[string]any{"url": gitFixtureURL, "ref": "", "sub_dir": "", "expected_commit": ""})
		if err != nil {
			t.Fatal(err)
		}
		doc["source_locator_digest"] = hash(canonical)
	})
}

func zipifyImportRecord(t *testing.T, f fixture) {
	t.Helper()
	rewriteImportRecord(t, f, func(doc map[string]any) {
		doc["schema_version"] = "local-skill-import/v2"
		doc["source_kind"] = "https_zip"
		delete(doc, "git")
		doc["remote"] = map[string]any{"archive_sha256": zipFixtureDigest, "archive_bytes": 4096, "final_locator_digest": locatorFixture, "archive_path": "", "expected_sha256": zipFixtureDigest}
		doc["excluded_git_metadata"] = true
		canonical, err := canon.Marshal(map[string]any{"url": zipFixtureURL, "archive_path": "", "expected_sha256": zipFixtureDigest})
		if err != nil {
			t.Fatal(err)
		}
		doc["source_locator_digest"] = hash(canonical)
	})
}

func installedSnapshot(t *testing.T, s *Store, id string) (dirs []string, files []skillimport.File) {
	t.Helper()
	_, claim, err := s.historicalRecord(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return claim.Directories, claim.Files
}

func storeTree(t *testing.T, root string) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return seen
}

func extraKeys(before, after map[string]bool) []string {
	var out []string
	for key := range after {
		if !before[key] {
			out = append(out, key)
		}
	}
	return out
}

func TestUpdateCheckReportsMovedUpstreamWithoutAuthority(t *testing.T) {
	f, op := installedInspection(t)
	s := f.store
	gitifyImportRecord(t, f)
	_, revisionBefore, err := s.authority.GetGrantWithSeq(f.request.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	s.upstream = func(ctx context.Context, rec *skillimport.Record, remoteURL string) (*skillimport.UpstreamSnapshot, error) {
		if rec.ImportID != f.importID || rec.SourceKind != "git" || remoteURL != "" {
			t.Error("wrong dispatch", rec.ImportID, rec.SourceKind, remoteURL)
		}
		dirs, files := installedSnapshot(t, s, op.InstallID)
		return &skillimport.UpstreamSnapshot{SourceKind: "git", URL: gitFixtureURL, CommitSHA: gitFixtureCommit, Directories: dirs, Files: files}, nil
	}
	before := storeTree(t, s.dir)
	same, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "human"})
	if err != nil {
		t.Fatal(err)
	}
	if same.Status != "up_to_date" || same.RequiresConfirmation || same.SourceKind != "git" || same.UpstreamCommitSHA != gitFixtureCommit || len(same.ContentChanges) != 0 || same.ContentChangesTotal != 0 || same.PermissionComparison != updateCheckPermissionDeferred || same.CheckedAt == "" || same.InstallID != op.InstallID {
		t.Fatal(same)
	}
	if after := storeTree(t, s.dir); len(extraKeys(before, after)) != 0 {
		t.Fatal("update check wrote state", extraKeys(before, after))
	}
	if _, revision, err := s.authority.GetGrantWithSeq(f.request.GrantID); err != nil || revision != revisionBefore {
		t.Fatal("update check touched authority", revision, revisionBefore)
	}
	// A moved upstream is reported, never applied: extra upstream content and
	// a new commit only set RequiresConfirmation for the explicit flow.
	s.upstream = func(ctx context.Context, rec *skillimport.Record, remoteURL string) (*skillimport.UpstreamSnapshot, error) {
		dirs, files := installedSnapshot(t, s, op.InstallID)
		next := make([]skillimport.File, len(files), len(files)+2)
		copy(next, files)
		next = append(next,
			skillimport.File{Path: "CHANGE.md", SHA256: strings.Repeat("1", 64), Bytes: 8},
			skillimport.File{Path: "payload.sh", SHA256: strings.Repeat("2", 64), Bytes: 12, Executable: true},
		)
		return &skillimport.UpstreamSnapshot{SourceKind: "git", URL: gitFixtureURL, CommitSHA: strings.Repeat("f", 40), Directories: dirs, Files: next}, nil
	}
	next, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "human"})
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != "new_version" || !next.RequiresConfirmation || next.UpstreamCommitSHA != strings.Repeat("f", 40) || next.ContentChangesTotal != 2 || len(next.ContentChanges) != 2 {
		t.Fatal(next)
	}
	for _, change := range next.ContentChanges {
		if change.Before != nil || change.After == nil {
			t.Fatal(change)
		}
	}
	if after := storeTree(t, s.dir); len(extraKeys(before, after)) != 0 {
		t.Fatal("update check wrote state", extraKeys(before, after))
	}
}

func TestUpdateCheckTruncatesLargeDiffs(t *testing.T) {
	f, op := installedInspection(t)
	s := f.store
	gitifyImportRecord(t, f)
	s.upstream = func(ctx context.Context, rec *skillimport.Record, remoteURL string) (*skillimport.UpstreamSnapshot, error) {
		dirs, files := installedSnapshot(t, s, op.InstallID)
		next := make([]skillimport.File, len(files), len(files)+maxInspectionChanges+1)
		copy(next, files)
		for i := 0; i <= maxInspectionChanges; i++ {
			next = append(next, skillimport.File{Path: "gen/" + strings.Repeat("x", i+1) + ".md", SHA256: strings.Repeat("a", 64), Bytes: int64(i + 1)})
		}
		return &skillimport.UpstreamSnapshot{SourceKind: "git", URL: gitFixtureURL, CommitSHA: gitFixtureCommit, Directories: dirs, Files: next}, nil
	}
	out, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "human"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "new_version" || !out.RequiresConfirmation || out.ContentChangesTotal != maxInspectionChanges+1 || len(out.ContentChanges) != maxInspectionChanges || !out.ContentChangesTruncated {
		t.Fatal(out.Status, out.ContentChangesTotal, len(out.ContentChanges), out.ContentChangesTruncated)
	}
}

func TestUpdateCheckGuards(t *testing.T) {
	f, op := installedInspection(t)
	s := f.store
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v2", ActorID: "human"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("schema accepted", err)
	}
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("empty actor accepted", err)
	}
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: " spaced "}); !errors.Is(err, ErrInvalid) {
		t.Fatal("padded actor accepted", err)
	}
	if _, err := s.CheckUpdate(context.Background(), "sin-"+strings.Repeat("9", 64), UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "human"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown install accepted", err)
	}
	// local_dir imports have no fetchable upstream.
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "human"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("local source accepted", err)
	}
	// Git sources refuse a caller URL; zip sources require one.
	gitifyImportRecord(t, f)
	s.upstream = func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
		t.Error("check fetched with a caller-supplied git URL")
		return nil, ErrInvalid
	}
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", RemoteURL: "https://evil.example.com/skill.git", ActorID: "human"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("git source accepted remote_url", err)
	}
	zipifyImportRecord(t, f)
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "human"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("zip source accepted empty remote_url", err)
	}
	// Tampering with the import artifact breaks the installed binding.
	rewriteImportRecord(t, f, func(doc map[string]any) {
		doc["artifact_digest"] = strings.Repeat("0", 64)
	})
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", RemoteURL: zipFixtureURL, ActorID: "human"}); !errors.Is(err, ErrChanged) {
		t.Fatal("broken artifact binding accepted", err)
	}
	// A pending removal blocks new checks.
	s.upstream = func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
		t.Error("check fetched while removal is pending")
		return nil, ErrInvalid
	}
	f.revoke(t)
	req := removalRequest(t, s, op.InstallID)
	if _, err := s.Remove(context.Background(), op.InstallID, req); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", RemoteURL: zipFixtureURL, ActorID: "human"}); !errors.Is(err, ErrRemovalPending) {
		t.Fatal("removal pending not reported", err)
	}
}
