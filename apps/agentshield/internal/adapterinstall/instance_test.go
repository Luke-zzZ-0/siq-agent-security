package adapterinstall

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	statepkg "siq-agent-security/apps/agentshield/internal/state"

	"siq-agent-security/apps/agentshield/internal/hermeshome"
)

func TestHermesInstancesHaveSeparatePlansAndUninstallRecords(t *testing.T) {
	opts := testOpts(t, Hermes)
	root := filepath.Join(opts.Home, ".hermes", "profiles", "work")
	putTestFile(t, filepath.Join(root, "config.yaml"), []byte("model: work\n"), 0600)
	roots := hermeshome.Scan(hermeshome.Options{Home: opts.Home, OS: "linux"}).Roots
	if len(roots) != 2 {
		t.Fatal("named profile not found")
	}
	first, second := WithHermesInstance(opts, roots[0]), WithHermesInstance(opts, roots[1])
	for _, o := range []Options{first, second} {
		p := testPlan(t, o, "install")
		if p.View().SchemaVersion != "local-adapter-plan/v2" || p.View().InstanceID != o.Instance.ID {
			t.Fatal("missing explicit target")
		}
		if _, err := Apply(p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Uninstall(second); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(first.configRoot(), "plugins", "siq-agent-security", "plugin.yaml")) {
		t.Fatal("other instance removed")
	}
	if exists(filepath.Join(second.configRoot(), "plugins", "siq-agent-security", "plugin.yaml")) {
		t.Fatal("selected instance not removed")
	}
	if _, err := Uninstall(first); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(opts.Home, ".local", "bin", "hermes-skills-install")) {
		t.Fatal("instance install changed shared wrapper")
	}
	raw, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil || string(raw) != "model: work\n" {
		t.Fatal("files-only installation changed host config")
	}
}

func TestDefaultInstanceUpgradeRestoresLegacyWrapper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX wrapper")
	}
	opts := testOpts(t, Hermes)
	legacy := opts.wrapperPath()
	putTestFile(t, legacy, []byte("#!/bin/sh\n# original user wrapper\n"), 0755)
	if _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	root := hermeshome.Scan(hermeshome.Options{Home: opts.Home, OS: "linux"}).Roots[0]
	targeted := WithHermesInstance(opts, root)
	if _, err := Install(targeted); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(targeted); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(legacy)
	info, statErr := os.Stat(legacy)
	if err != nil || statErr != nil || string(raw) != "#!/bin/sh\n# original user wrapper\n" || info.Mode().Perm() != 0755 {
		t.Fatal("legacy user wrapper was not restored")
	}
	if exists(targeted.wrapperPath()) {
		t.Fatal("new instance wrapper not removed")
	}
}

func TestPlanRejectsConfigChangedBetweenPrepareAndApply(t *testing.T) {
	opts := testOpts(t, Hermes)
	configJSON := filepath.Join(opts.configRoot(), "plugins", "siq-agent-security", "config.json")
	putTestFile(t, configJSON, []byte(`{"endpoint":"http://127.0.0.1:47611"}`), 0600)
	p := testPlan(t, opts, "install")
	if len(p.payload.Inputs) == 0 {
		t.Fatal("plan captured no before-images")
	}
	if _, err := Apply(p); err != nil {
		t.Fatalf("unchanged configuration must apply cleanly: %v", err)
	}
	if _, err := Uninstall(opts); err != nil {
		t.Fatal(err)
	}
	stale := testPlan(t, opts, "install")
	if err := os.WriteFile(configJSON, []byte(`{"endpoint":"http://127.0.0.1:47612"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(stale); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("stale plan must be rejected with ErrPlanChanged, got %v", err)
	}
}

func testOptsAt(t *testing.T, home string) Options {
	t.Helper()
	state := t.TempDir()
	if _, err := statepkg.Open(state); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, "bin", "agentshield")
	putTestFile(t, bin, []byte("#!/bin/sh\n"), 0o700)
	return Options{
		Platform: Hermes,
		Home:     home,
		StateDir: state,
		Binary:   bin,
		Endpoint: "http://127.0.0.1:47611",
		Mode:     "block",
		Now:      time.Date(2026, 9, 4, 6, 0, 0, 0, time.UTC),
	}
}

func TestPlanPinnedToMovedInstanceIsRejected(t *testing.T) {
	home := t.TempDir()
	profile := filepath.Join(home, ".hermes", "profiles", "work")
	putTestFile(t, filepath.Join(profile, "config.yaml"), []byte("model: work\n"), 0600)
	roots := hermeshome.Scan(hermeshome.Options{Home: home, OS: "linux"}).Roots
	var target hermeshome.Root
	for _, root := range roots {
		if root.Path == profile {
			target = root
		}
	}
	if target.ID == "" {
		t.Fatal("named profile not discovered")
	}
	opts := WithHermesInstance(testOptsAt(t, home), target)
	p := testPlan(t, opts, "install")

	if err := os.Rename(profile, profile+"-moved"); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(p); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("plan on moved instance must be rejected, got %v", err)
	}

	// Identity is path-derived: a directory recreated at the old path is the
	// same instance identity, so the binding stays with the path — but only a
	// freshly prepared plan may act on it.
	putTestFile(t, filepath.Join(profile, "config.yaml"), []byte("model: recreated\n"), 0600)
	fresh := testPlan(t, WithHermesInstance(testOptsAt(t, home), target), "install")
	if _, err := Apply(fresh); err != nil {
		t.Fatalf("plan prepared against the current instance must apply: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(profile, "config.yaml"))
	if err != nil || string(raw) != "model: recreated\n" {
		t.Fatal("fresh install modified host config")
	}
}
