package adapterinstall

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/hermeshome"
	"siq-agent-security/apps/agentshield/internal/product"
)

func managedTestOptions(t *testing.T) Options {
	t.Helper()
	o := testOpts(t, Hermes)
	root := filepath.Join(o.Home, ".hermes", "profiles", "work")
	o.Instance = &InstanceTarget{ID: hermeshome.Identifier(root), Name: "work", ConfigDir: root}
	o.RuntimeIdentityID = "ri-" + strings.Repeat("a", 32)
	// This is only a file-transaction fixture; signature verification is a server/core responsibility.
	raw, _ := json.Marshal(map[string]any{"identity_id": o.RuntimeIdentityID, "instance_id": o.Instance.ID, "agent_id": "hri-" + strings.TrimPrefix(o.Instance.ID, "hi-"), "platform": Hermes})
	putTestFile(t, filepath.Join(o.StateDir, "runtime-identities", o.RuntimeIdentityID+".json"), raw, 0600)
	return o
}
func TestManagedPlanPinsPublicIdentityInputsWithoutReadingSecret(t *testing.T) {
	for _, change := range []string{"metadata", "revocation", "secret"} {
		t.Run(change, func(t *testing.T) {
			o := managedTestOptions(t)
			p := testPlan(t, o, "install") // No secret file exists: planning must not need its contents.
			for path := range p.payload.Inputs {
				if strings.Contains(path, "runtime-identity-secrets") {
					t.Fatal("secret captured in journal inputs")
				}
			}
			switch change {
			case "metadata":
				putTestFile(t, filepath.Join(o.StateDir, "runtime-identities", o.RuntimeIdentityID+".json"), []byte("changed metadata"), 0600)
			case "revocation":
				putTestFile(t, managedRevocationPath(o), []byte("revoked"), 0600)
			case "secret":
				putTestFile(t, managedCredentialPath(o), []byte("synthetic-secret-not-journaled"), 0600)
			}
			_, err := Apply(p)
			if change != "secret" {
				if !errors.Is(err, ErrPlanChanged) {
					t.Fatal("stale identity plan accepted", err)
				}
				if exists(filepath.Join(o.configRoot(), "plugins")) {
					t.Fatal("stale plan wrote host")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(p.payload)
			if strings.Contains(string(raw), "synthetic-secret-not-journaled") {
				t.Fatal("credential captured")
			}
		})
	}
}
func TestManagedConfigurationCannotBeSilentlyDowngraded(t *testing.T) {
	o := managedTestOptions(t)
	config := filepath.Join(o.configRoot(), "plugins", "siq-agent-security", "config.json")
	raw, _ := json.Marshal(map[string]any{"runtime_identity_id": o.RuntimeIdentityID})
	putTestFile(t, config, raw, 0600)
	o.RuntimeIdentityID = ""
	if _, err := Prepare(o, "install"); !errors.Is(err, ErrPlanChanged) {
		t.Fatal("unowned managed config downgraded", err)
	}
}

// OpenClaw keeps its managed connection fields in the product config at the
// instance root, using camelCase keys per that file's host convention.
func openClawManagedTestOptions(t *testing.T) Options {
	t.Helper()
	o := testOpts(t, OpenClaw)
	root := filepath.Join(o.Home, ".openclaw")
	o.Instance = &InstanceTarget{ID: hermeshome.Identifier(root), Name: "default", ConfigDir: root}
	o.RuntimeIdentityID = "ri-" + strings.Repeat("b", 32)
	raw, _ := json.Marshal(map[string]any{"identity_id": o.RuntimeIdentityID, "instance_id": o.Instance.ID, "agent_id": "hri-" + strings.TrimPrefix(o.Instance.ID, "hi-"), "platform": OpenClaw})
	putTestFile(t, filepath.Join(o.StateDir, "runtime-identities", o.RuntimeIdentityID+".json"), raw, 0600)
	return o
}

func TestOpenClawManagedPlanWritesProductConfigFields(t *testing.T) {
	o := openClawManagedTestOptions(t)
	p := testPlan(t, o, "install")
	if _, err := Apply(p); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(o.configRoot(), product.Name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if json.Unmarshal(raw, &cfg) != nil {
		t.Fatal("product config not valid JSON")
	}
	agent := "hri-" + strings.TrimPrefix(o.Instance.ID, "hi-")
	if cfg["runtimeIdentityId"] != o.RuntimeIdentityID || cfg["agentId"] != agent || cfg["tokenPath"] != managedCredentialPath(o) {
		t.Fatalf("managed fields not pinned to signed identity: %v", cfg)
	}
	if _, err := os.Stat(filepath.Join(o.configRoot(), "plugins", product.PluginDir(), "index.ts")); err != nil {
		t.Fatal("plugin assets missing")
	}
	id, err := ConfiguredRuntimeIdentity(o)
	if err != nil || id != o.RuntimeIdentityID {
		t.Fatalf("configured identity round-trip: %q %v", id, err)
	}
	d := Inspect(o)
	for _, code := range []string{"adapter_files", "service_configuration", "host_registration"} {
		if checkStatus(d, code) != "pass" {
			t.Fatalf("diagnosis %s = %+v", code, d.Checks)
		}
	}
}

func TestOpenClawManagedConfigurationCannotBeSilentlyDowngraded(t *testing.T) {
	o := openClawManagedTestOptions(t)
	o.RuntimeIdentityID = ""
	raw, _ := json.Marshal(map[string]any{"runtimeIdentityId": "ri-" + strings.Repeat("b", 32)})
	putTestFile(t, filepath.Join(o.configRoot(), product.Name+".json"), raw, 0600)
	if _, err := Prepare(o, "install"); !errors.Is(err, ErrPlanChanged) {
		t.Fatal("unowned managed product config downgraded", err)
	}
}

func TestOpenClawManagedPinRejectsForeignPlatformIdentity(t *testing.T) {
	o := openClawManagedTestOptions(t)
	raw, _ := json.Marshal(map[string]any{"identity_id": o.RuntimeIdentityID, "instance_id": o.Instance.ID, "agent_id": "hri-" + strings.TrimPrefix(o.Instance.ID, "hi-"), "platform": Hermes})
	putTestFile(t, filepath.Join(o.StateDir, "runtime-identities", o.RuntimeIdentityID+".json"), raw, 0600)
	if _, err := Prepare(o, "install"); !errors.Is(err, ErrPlanChanged) {
		t.Fatal("hermes-signed identity accepted by openclaw target", err)
	}
}

func TestOpenClawManagedUninstallRequiresIdentityWithdrawal(t *testing.T) {
	o := openClawManagedTestOptions(t)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(o); !errors.Is(err, ErrIdentityWithdrawalRequired) {
		t.Fatal("active managed credential removable without revocation", err)
	}
	if _, err := os.Stat(filepath.Join(o.configRoot(), product.Name+".json")); err != nil {
		t.Fatal("blocked uninstall removed host config")
	}
	putTestFile(t, managedRevocationPath(o), []byte(`{"revoked":true}`), 0600)
	if _, err := Uninstall(o); err != nil {
		t.Fatal("published revocation did not unlock uninstall", err)
	}
}
