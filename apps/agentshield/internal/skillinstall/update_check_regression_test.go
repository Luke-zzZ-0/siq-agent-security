package skillinstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/skillimport"
)

func stateFingerprint(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = info.Mode().String()
		if entry.Type().IsRegular() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			out[rel] += hex.EncodeToString(sum[:])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestUpdateCheckSampleAndNoPersistentWrites(t *testing.T) {
	f, op := installedInspection(t)
	zipifyImportRecord(t, f)
	f.store.now = func() time.Time { return time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC) }
	f.store.upstream = func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
		dirs, files := installedSnapshot(t, f.store, op.InstallID)
		files = append(append([]skillimport.File(nil), files...), skillimport.File{Path: "CHANGE.md", SHA256: strings.Repeat("b", 64), Bytes: 8})
		return &skillimport.UpstreamSnapshot{SourceKind: "https_zip", ArchiveSHA256: strings.Repeat("c", 64), Directories: dirs, Files: files}, nil
	}
	before := stateFingerprint(t, filepath.Dir(f.store.dir))
	req := UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", RemoteURL: zipFixtureURL, ActorID: "fixture-reviewer"}
	got, err := f.store.CheckUpdate(context.Background(), op.InstallID, req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/contracts/local-skill-update-check-result.json")
	if err != nil {
		t.Fatal(err)
	}
	var want UpdateCheckResult
	if err = json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	want.InstallID = op.InstallID
	if !reflect.DeepEqual(got, &want) {
		t.Fatalf("result differs from contract sample: %#v", got)
	}
	if !reflect.DeepEqual(before, stateFingerprint(t, filepath.Dir(f.store.dir))) {
		t.Fatal("state content, mode, or paths changed")
	}
	f.store.upstream = func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
		return nil, skillimport.ErrDownloadFailed
	}
	if _, err = f.store.CheckUpdate(context.Background(), op.InstallID, req); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = f.store.CheckUpdate(ctx, op.InstallID, req); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, stateFingerprint(t, filepath.Dir(f.store.dir))) {
		t.Fatal("failed/cancelled check changed state")
	}
}

func TestUpdateFetchErrorCategories(t *testing.T) {
	for _, tc := range []struct{ input, want error }{
		{skillimport.ErrSourceUnavailable, ErrUpdateSourceUnavailable},
		{skillimport.ErrGitHostUnsupported, ErrUpdateURLBlocked},
		{skillimport.ErrURLBlocked, ErrUpdateURLBlocked}, {skillimport.ErrChanged, ErrChanged},
		{skillimport.ErrLimit, ErrLimit}, {context.DeadlineExceeded, context.DeadlineExceeded},
		{skillimport.ErrDownloadFailed, ErrUnavailable}, {skillimport.ErrUnavailable, ErrUnavailable},
		{errors.New("private upstream diagnostic"), ErrUnavailable},
	} {
		if got := updateFetchError(context.Background(), tc.input); !errors.Is(got, tc.want) {
			t.Fatalf("%v => %v", tc.input, got)
		}
	}
}
