package server

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestScenarioConflictPreservesLiveGrantAndIdempotence(t *testing.T) {
	for _, initial := range []string{"", "no-exec", "sandboxed"} {
		for _, status := range []string{"pending_approval", "approved", "deployed"} {
			t.Run(initial+"/"+status, func(t *testing.T) {
				s, _ := newServer(t, "block")
				skill, _ := filepath.Abs(filepath.Join("..", "admission", "testdata", "skills", "benign", "pure-doc"))
				_, adm := call(t, s, "POST", "/v1/admit", token, map[string]any{"path": skill})
				body := map[string]any{"admission_id": adm["admission"].(map[string]any)["admission_id"], "platform": "hermes", "subject_id": "agent", "scenario_id": initial}
				code, out := call(t, s, "POST", "/v1/grants", token, body)
				if code != 200 {
					t.Fatal(code, out)
				}
				gid := out["grant"].(map[string]any)["grant_id"].(string)
				if status != "pending_approval" {
					code, out = approveChallenged(t, s, gid, "reviewer", stateRevision(t, out))
					if code != 200 {
						t.Fatal(code, out)
					}
				}
				if status == "deployed" {
					code, out = call(t, s, "POST", "/v1/grants/"+gid+"/deploy", token, withRevision(map[string]any{}, stateRevision(t, out)))
					if code != 200 {
						t.Fatal(code, out)
					}
				}
				before, rev, err := s.d.Store.GetGrantWithSeq(gid)
				if err != nil {
					t.Fatal(err)
				}
				code, out = call(t, s, "POST", "/v1/grants", token, body)
				if code != 200 || out["reused"] != true {
					t.Fatal("identical intent not idempotent", code, out)
				}
				for _, other := range []string{"", "no-exec", "sandboxed"} {
					if other == initial {
						continue
					}
					body["scenario_id"] = other
					code, out = call(t, s, "POST", "/v1/grants", token, body)
					if code != 409 || out["error"] != "grant_scenario_conflict" {
						t.Fatal("changed intent reused", code, out)
					}
					after, afterRev, err := s.d.Store.GetGrantWithSeq(gid)
					if err != nil || rev != afterRev || !reflect.DeepEqual(before, after) {
						t.Fatal("conflict changed grant or revision")
					}
				}
			})
		}
	}
}

func TestReviewerChangingScenarioIsNotSilentlyIgnored(t *testing.T) {
	s, _ := newServer(t, "block")
	skill, _ := filepath.Abs(filepath.Join("..", "admission", "testdata", "skills", "benign", "pure-doc"))
	code, admitted := call(t, s, "POST", "/v1/admit", token, map[string]any{"path": skill})
	if code != 200 {
		t.Fatal(code, admitted)
	}
	admID := admitted["admission"].(map[string]any)["admission_id"].(string)
	body := map[string]any{"admission_id": admID, "platform": "hermes", "subject_id": "same-agent"}
	code, plain := call(t, s, "POST", "/v1/grants", token, body)
	if code != 200 {
		t.Fatal(code, plain)
	}
	body["scenario_id"] = "no-exec"
	code, restricted := call(t, s, "POST", "/v1/grants", token, body)
	if code == 409 {
		return
	} // Explicit conflict is safe; silent reuse is not.
	if code != 200 {
		t.Fatal(code, restricted)
	}
	g := restricted["grant"].(map[string]any)
	sc, _ := g["scenario"].(map[string]any)
	if sc == nil || sc["id"] != "no-exec" {
		t.Fatalf("requested no-exec but got reused=%v scenario=%v status=%v", restricted["reused"], g["scenario"], g["status"])
	}
}
