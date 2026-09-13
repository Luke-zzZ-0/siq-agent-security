package rawcontent

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/signing"
)

func TestActivationIsExplicitPersistentAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	key, _ := signing.FromSeed(bytes.Repeat([]byte{2}, 32))
	limits := Limits{Retention: 6 * time.Hour, Budget: 8 << 20}
	if _, _, _, err := OpenActivated(dir, key); !errors.Is(err, ErrDisabled) {
		t.Fatal("missing activation not disabled", err)
	}
	if _, err := os.Stat(keyPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("open created encryption key", err)
	}
	if _, _, _, err := Activate(dir, key, limits, "", time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid actor initialized store", err)
	}
	if _, err := os.Stat(keyPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid activation created encryption key", err)
	}
	now := time.Date(2026, 9, 13, 7, 0, 0, 0, time.UTC)
	store, authority, activation, err := Activate(dir, key, limits, "PRIVATE_ACTOR", now)
	if err != nil || store == nil || authority == nil || !activation.Enabled {
		t.Fatal("activate", activation, err)
	}
	if activation.RetentionSeconds != 21600 || activation.BudgetBytes != 8<<20 || !signing.VerifyCanonical(key.Public(), activationMap(activation), activation.Signature) {
		t.Fatal("invalid signed activation", activation)
	}
	raw, err := os.ReadFile(activationPath(dir))
	if err != nil || bytes.Contains(raw, []byte("PRIVATE_ACTOR")) {
		t.Fatal("actor plaintext persisted", err)
	}
	openedStore, openedAuthority, opened, err := OpenActivated(dir, key)
	if err != nil || openedStore.KeyFingerprint() != store.KeyFingerprint() || openedAuthority == nil || opened != activation {
		t.Fatal("restart open", opened, err)
	}
	_, _, retry, err := Activate(dir, key, limits, "different-retry-actor", now.Add(time.Hour))
	if err != nil || retry != activation {
		t.Fatal("activation retry replaced record", retry, err)
	}
	if _, _, _, err := Activate(dir, key, Limits{Retention: 7 * time.Hour, Budget: 8 << 20}, "actor", now); !errors.Is(err, ErrConflict) {
		t.Fatal("limit change replaced immutable activation", err)
	}
	wrongKey, _ := signing.FromSeed(bytes.Repeat([]byte{3}, 32))
	if _, _, _, err := OpenActivated(dir, wrongKey); !errors.Is(err, ErrState) {
		t.Fatal("wrong signing identity accepted", err)
	}
}

func TestTamperedOrPartialActivationFailsClosed(t *testing.T) {
	key, _ := signing.FromSeed(bytes.Repeat([]byte{4}, 32))
	dir := t.TempDir()
	store, err := Initialize(dir, Limits{Retention: DefaultRetention, Budget: DefaultBudget})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InitializeAuthority(dir, key, store); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := OpenActivated(dir, key); !errors.Is(err, ErrDisabled) {
		t.Fatal("partial initialization became active", err)
	}
	now := time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)
	_, _, activation, err := Activate(dir, key, Limits{Retention: DefaultRetention, Budget: DefaultBudget}, "actor", now)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(activationPath(dir))
	var document map[string]any
	_ = json.Unmarshal(raw, &document)
	document["budget_bytes"] = float64(2 << 20)
	tampered, _ := json.Marshal(document)
	if err := os.WriteFile(activationPath(dir), tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := OpenActivated(dir, key); !errors.Is(err, ErrState) {
		t.Fatal("tampered activation opened", err)
	}
	if _, _, _, err := Activate(dir, key, Limits{Retention: DefaultRetention, Budget: DefaultBudget}, "actor", now); !errors.Is(err, ErrState) {
		t.Fatal("tampered activation overwritten", activation, err)
	}
}

func TestActivationFixture(t *testing.T) {
	oldRandom := random
	random = bytes.NewReader(bytes.Repeat([]byte{5}, 32))
	t.Cleanup(func() { random = oldRandom })
	dir := t.TempDir()
	key, _ := signing.FromSeed(bytes.Repeat([]byte{9}, 32))
	_, _, activation, err := Activate(
		dir,
		key,
		Limits{Retention: DefaultRetention, Budget: DefaultBudget},
		"fixture-operator",
		time.Date(2026, 9, 13, 2, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(activation, "", "  ")
	raw = append(raw, '\n')
	want, err := os.ReadFile(filepath.Join("../../testdata/contracts", "local-raw-task-content-activation.json"))
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatalf("activation cross-language fixture differs: %v\n%s", err, raw)
	}
}
