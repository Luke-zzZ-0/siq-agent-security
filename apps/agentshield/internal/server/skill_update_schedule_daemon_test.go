package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func stateTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The daemon maintenance entry point must behave exactly like the documented
// scheduler contract on a live server: an enabled entry whose next check is
// in the future is counted and left untouched, a disabled entry is counted
// without a fetch, and the pass never writes outside the schedule records.
func TestRunScheduledUpdateChecksDaemonEntry(t *testing.T) {
	s, importID, installID := updateSourceFixture(t)
	zipifyServerImportRecord(t, s, importID)
	route := "/v1/skill-installations/operations/" + installID + "/update-source"
	code, saved := call(t, s, "POST", route, token, updateSourceSaveBody(serverZipFixtureURL, true))
	if code != 200 {
		t.Fatal(code, saved)
	}
	before := stateTreeSnapshot(t, s.d.Store.Dir)

	res, err := s.RunScheduledUpdateChecks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// SaveUpdateSource pushed next_check_at a full interval out, so the entry
	// is not due and no upstream fetch may happen.
	if res.SchemaVersion != "local-skill-update-schedule-run-result/v1" || res.Total != 1 || res.Due != 0 || len(res.Checked) != 0 || res.Deferred != 0 || res.Stale != 0 || res.Disabled != 0 {
		t.Fatal("not-due counts", res)
	}
	if after := stateTreeSnapshot(t, s.d.Store.Dir); len(after) != len(before) {
		t.Fatal("daemon pass changed the state file set")
	} else {
		for path, sum := range before {
			if after[path] != sum {
				t.Fatal("daemon pass rewrote", path)
			}
		}
	}

	code, disabled := call(t, s, "POST", route, token, updateSourceSaveBody(serverZipFixtureURL, false))
	if code != 200 {
		t.Fatal(code, disabled)
	}
	before = stateTreeSnapshot(t, s.d.Store.Dir)
	res, err = s.RunScheduledUpdateChecks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Disabled != 1 || res.Due != 0 || len(res.Checked) != 0 {
		t.Fatal("disabled counts", res)
	}
	if after := stateTreeSnapshot(t, s.d.Store.Dir); len(after) != len(before) {
		t.Fatal("daemon pass changed the state file set")
	} else {
		for path, sum := range before {
			if after[path] != sum {
				t.Fatal("daemon pass rewrote", path)
			}
		}
	}
}
