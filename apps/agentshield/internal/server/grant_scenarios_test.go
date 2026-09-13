package server

import (
	"path/filepath"
	"testing"
)

func TestGrantScenarioCatalogEndpoint(t *testing.T) {
	s, _ := newServer(t, "block")
	code, out := call(t, s, "GET", "/v1/grant-scenarios", token, nil)
	if code != 200 {
		t.Fatalf("catalog: %d %v", code, out)
	}
	list, _ := out["scenarios"].([]any)
	if len(list) != 3 {
		t.Fatalf("expected closed catalog of 3, got %d", len(list))
	}
	ids := map[string]bool{}
	for _, entry := range list {
		m, _ := entry.(map[string]any)
		ids[m["id"].(string)] = true
		for _, secretKey := range []string{"secret", "token", "key", "signature"} {
			if _, present := m[secretKey]; present {
				t.Fatalf("catalog entry carries sensitive field %q", secretKey)
			}
		}
	}
	for _, want := range []string{"no-network", "no-exec", "sandboxed"} {
		if !ids[want] {
			t.Fatalf("catalog missing %s", want)
		}
	}
	code, _ = call(t, s, "POST", "/v1/grant-scenarios", token, map[string]any{})
	if code != 405 {
		t.Fatalf("catalog must be read-only: %d", code)
	}
}

func TestGrantCreateWithScenarioRestrictsAndBinds(t *testing.T) {
	s, _ := newServer(t, "block")
	skill, _ := filepath.Abs(filepath.Join("..", "admission", "testdata", "skills", "benign", "pure-doc"))
	_, a := call(t, s, "POST", "/v1/admit", token, map[string]any{"path": skill})
	admID := a["admission"].(map[string]any)["admission_id"].(string)

	// Unknown scenario must be rejected before any grant is derived.
	code, errOut := call(t, s, "POST", "/v1/grants", token, map[string]any{
		"admission_id": admID, "platform": "hermes", "subject_id": "sc", "scenario_id": "total-access"})
	if code != 400 {
		t.Fatalf("unknown scenario must 400: %d %v", code, errOut)
	}

	code, created := call(t, s, "POST", "/v1/grants", token, map[string]any{
		"admission_id": admID, "platform": "hermes", "subject_id": "sc", "scenario_id": "sandboxed"})
	if code != 200 {
		t.Fatalf("create: %d %v", code, created)
	}
	g := created["grant"].(map[string]any)
	sc, _ := g["scenario"].(map[string]any)
	if sc == nil || sc["id"] != "sandboxed" || sc["version"].(float64) != 1 {
		t.Fatalf("scenario binding missing: %v", sc)
	}
	facts, _ := g["facts"].([]any)
	for _, f := range facts {
		fact, _ := f.(map[string]any)
		if fact["domain"] == "network" || fact["domain"] == "process" || fact["domain"] == "resource" {
			t.Fatalf("scenario grant kept %v fact", fact["domain"])
		}
		if fact["domain"] == "filesystem" && fact["action"] == "fs.write" {
			t.Fatal("scenario grant kept fs.write")
		}
	}
	if g["default_effect"] != "deny" {
		t.Fatal("scenario grant must keep default deny")
	}

	// Same admission without a scenario keeps every declared fact: the
	// scenario only narrows, never widens.
	code, plain := call(t, s, "POST", "/v1/grants", token, map[string]any{
		"admission_id": admID, "platform": "hermes", "subject_id": "sc2"})
	if code != 200 {
		t.Fatalf("baseline create: %d %v", code, plain)
	}
	if _, has := plain["grant"].(map[string]any)["scenario"]; has {
		t.Fatal("baseline grant must not carry a scenario binding")
	}
}
