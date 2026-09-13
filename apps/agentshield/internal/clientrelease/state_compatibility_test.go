package clientrelease

import (
	"os"
	"path/filepath"
	"runtime"
	"siq-agent-security/apps/agentshield/internal/skillmanifest"
	"siq-agent-security/apps/agentshield/internal/state"
	"testing"
)

func TestStateBoundReleaseBeforeAnyStaging(t *testing.T) {
	dir := t.TempDir()
	s, e := state.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	w, e := state.AcquireWriter(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Initialize(w, 49123); e != nil {
		t.Fatal(e)
	}
	w.Release()
	manifest, binary, m, key := fixture(t)
	m.ManifestVersion = 2
	m.ClientCompatibility = skillmanifest.CurrentClientCompatibility()
	writeManifest(t, manifest, m, key)
	if _, e := checkUpgrade(manifest, binary, runtime.GOOS, runtime.GOARCH, key.Public(), dir); e == nil {
		t.Fatal("old release approved for v2")
	}
	if _, e := stage(dir, manifest, binary, runtime.GOOS, runtime.GOARCH, key.Public()); e == nil {
		t.Fatal("old release staged for v2")
	}
	if _, e := os.Lstat(filepath.Join(dir, "client-releases")); !os.IsNotExist(e) {
		t.Fatal("rejected candidate wrote staging")
	}
	m.ManifestVersion = 3
	m.StateCompatibility = skillmanifest.CurrentStateCompatibility()
	writeManifest(t, manifest, m, key)
	if _, e := checkUpgrade(manifest, binary, runtime.GOOS, runtime.GOARCH, key.Public(), dir); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"reader", "writer", "format"} {
		m.StateCompatibility = skillmanifest.CurrentStateCompatibility()
		switch kind {
		case "reader":
			m.StateCompatibility.ReaderVersion = 1
		case "writer":
			m.StateCompatibility.ReaderVersion = 1
			m.StateCompatibility.WriterVersion = 1
		case "format":
			m.StateCompatibility.MaxFormat = 1
		}
		writeManifest(t, manifest, m, key)
		if _, e := checkUpgrade(manifest, binary, runtime.GOOS, runtime.GOARCH, key.Public(), dir); e == nil {
			t.Fatal("unsupported candidate accepted", kind)
		}
	}
}
