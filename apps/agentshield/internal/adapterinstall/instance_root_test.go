package adapterinstall

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"siq-agent-security/apps/agentshield/internal/hermeshome"
)

func rootBoundOptions(t *testing.T) (Options, string) {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, ".hermes", "profiles", "团队 work")
	putTestFile(t, filepath.Join(root, "config.yaml"), []byte("model: work\n"), 0600)
	for _, candidate := range hermeshome.Scan(hermeshome.Options{Home: home, OS: "linux"}).Roots {
		if candidate.Path == root {
			return WithHermesInstance(testOptsAt(t, home), candidate), root
		}
	}
	t.Fatal("profile not discovered")
	return Options{}, ""
}

func TestPlanRejectsRecreatedInstanceRoot(t *testing.T) {
	for _, content := range []string{"model: work\n", "model: replacement\n"} {
		t.Run(content, func(t *testing.T) {
			opts, root := rootBoundOptions(t)
			plan := testPlan(t, opts, "install")
			if err := os.Rename(root, root+"-moved"); err != nil {
				t.Fatal(err)
			}
			// Do not first Apply while the root is missing: rejection must come
			// from the preview's identity binding, not a remembered failed Apply.
			putTestFile(t, filepath.Join(root, "config.yaml"), []byte(content), 0600)
			if _, err := Apply(plan); !errors.Is(err, ErrPlanChanged) {
				t.Fatalf("old plan applied to replacement root: %v", err)
			}
			for _, dir := range []string{root, root + "-moved"} {
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 1 || entries[0].Name() != "config.yaml" {
					t.Fatalf("rejected plan wrote to root %q: %v, %v", dir, entries, err)
				}
			}
			for _, change := range plan.payload.Files {
				current, err := readImage(opts.Home, change.Path)
				if err != nil || !sameImage(current, change.Before) {
					t.Fatalf("rejected plan changed target %q: %v", change.Path, err)
				}
			}
			// The replacement is still usable after a fresh explicit preview.
			fresh := testPlan(t, opts, "install")
			if _, err := Apply(fresh); err != nil {
				t.Fatalf("fresh plan rejected: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(root, "config.yaml"))
			if err != nil || !bytes.Equal(raw, []byte(content)) {
				t.Fatal("fresh install modified host configuration", err)
			}
		})
	}
}

func TestPlanRejectsInstanceRootSymlink(t *testing.T) {
	opts, root := rootBoundOptions(t)
	plan := testPlan(t, opts, "install")
	if err := os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"-moved", root); err != nil {
		t.Skipf("symlink unavailable on this host: %v", err)
	}
	if _, err := Apply(plan); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("symlink to original root accepted: %v", err)
	}
}

func TestPlanRootBindingIsRequiredForApply(t *testing.T) {
	opts, _ := rootBoundOptions(t)
	plan := testPlan(t, opts, "install")
	// A recovery payload cannot be promoted to a fresh pending preview.
	reconstructed := &Plan{payload: plan.payload}
	if _, err := Apply(reconstructed); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("plan without root identity accepted: %v", err)
	}
	// Ordinary directory mtime changes do not invalidate the root identity.
	putTestFile(t, filepath.Join(opts.Instance.ConfigDir, "notes.txt"), []byte("user notes"), 0600)
	if _, err := Apply(plan); err != nil {
		t.Fatalf("unchanged directory identity rejected: %v", err)
	}
}
