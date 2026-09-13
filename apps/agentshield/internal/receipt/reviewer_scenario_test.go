package receipt

import (
	"siq-agent-security/apps/agentshield/internal/admission"
	"siq-agent-security/apps/agentshield/internal/grant"
	"siq-agent-security/apps/agentshield/internal/signing"
	"strings"
	"testing"
)

// Regression cases retained from the independent stage acceptance.
func TestReviewerScenarioActuallyDeniesExec(t *testing.T) {
	for _, platform := range []string{"hermes", "openclaw"} {
		for _, scenario := range []string{"", "no-exec", "sandboxed"} {
			t.Run(platform+"/"+scenario, func(t *testing.T) {
				tool := "terminal"
				if platform == "openclaw" {
					tool = "exec"
				}
				adm := admission.Admission{AdmissionID: "adm-review", ContentHash: strings.Repeat("a", 64), Verdict: "admit_with_conditions", EvidenceIDs: []string{"ev-1"}, DeclaredFacts: []admission.DeclaredFact{
					{Domain: "tool", Action: "tool.invoke", Resource: admission.Resource{Type: "tool", Value: tool}, Effect: "allow", State: "declared", Authority: "skill_manifest", EvidenceIDs: []string{"ev-1"}},
					{Domain: "process", Action: "process.exec", Resource: admission.Resource{Type: "tool", Value: "shell"}, Effect: "allow", State: "declared", Authority: "skill_manifest", EvidenceIDs: []string{"ev-1"}},
				}}
				k, _ := signing.FromSeed([]byte(strings.Repeat("r", 32)))
				result, err := grant.Build(adm, grant.Options{Platform: platform, Subject: grant.Subject{Type: "agent_instance", ID: "inst_1"}, Key: k, Scenario: grant.ScenarioByID(scenario)})
				if err != nil {
					t.Fatal(err)
				}
				g, err := grant.Approve(result.Grant, grant.Approval{ActorType: "human", ActorID: "reviewer", ApprovedAt: "2026-09-13T00:00:00Z"}, k)
				if err != nil {
					t.Fatal(err)
				}
				g, err = grant.MarkDeployed(g, k)
				if err != nil {
					t.Fatal(err)
				}
				fx := newFixture(t, "block", &g, false)
				decision, err := fx.eng.Decide(req(platform, tool, map[string]any{"command": "printf review-marker"}))
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("scenario=%q action=%s reason=%s", scenario, decision.Action, decision.Reason)
				if scenario != "" && decision.Action != ActionDeny {
					t.Fatalf("scenario %s permits process execution: %s", scenario, decision.Action)
				}
			})
		}
	}
}

func TestScenarioKeepsAuthorizedReadAndRejectsRestrictedEffects(t *testing.T) {
	for _, platform := range []string{"hermes", "openclaw"} {
		for _, scenario := range []string{"no-exec", "no-network", "sandboxed"} {
			for _, tool := range []string{"read_file", "write_file", "web_fetch", "send_message", "custom-tool", "bash"} {
				t.Run(platform+"/"+scenario+"/"+tool, func(t *testing.T) {
					f := func(domain, action, kind, value string) admission.DeclaredFact {
						return admission.DeclaredFact{Domain: domain, Action: action, Resource: admission.Resource{Type: kind, Value: value}, Effect: "allow", State: "declared", Authority: "skill_manifest", EvidenceIDs: []string{"ev-1"}}
					}
					adm := admission.Admission{AdmissionID: "adm-review", ContentHash: strings.Repeat("a", 64), Verdict: "admit_with_conditions", EvidenceIDs: []string{"ev-1"}, DeclaredFacts: []admission.DeclaredFact{
						f("tool", "tool.invoke", "tool", tool), f("filesystem", "fs.read", "path", "/home/u/proj"), f("filesystem", "fs.write", "path", "/home/u/out"), f("network", "http.request", "endpoint", "api.github.com:443"),
					}}
					k, _ := signing.FromSeed([]byte(strings.Repeat("r", 32)))
					result, err := grant.Build(adm, grant.Options{Platform: platform, Subject: grant.Subject{Type: "agent_instance", ID: "inst_1"}, Key: k, Scenario: grant.ScenarioByID(scenario)})
					if err != nil {
						t.Fatal(err)
					}
					g, err := grant.Approve(result.Grant, grant.Approval{ActorType: "human", ActorID: "reviewer", ApprovedAt: "2026-09-13T00:00:00Z"}, k)
					if err != nil {
						t.Fatal(err)
					}
					g, err = grant.MarkDeployed(g, k)
					if err != nil {
						t.Fatal(err)
					}
					params := map[string]any{"path": "/home/u/proj/r.txt"}
					if tool == "write_file" {
						params["path"] = "/home/u/out/w.txt"
					}
					if tool == "web_fetch" {
						params = map[string]any{"url": "https://api.github.com/v1"}
					}
					if tool == "bash" {
						params = map[string]any{"command": "printf marker"}
					}
					fx := newFixture(t, "block", &g, false)
					d, err := fx.eng.Decide(req(platform, tool, params))
					if err != nil {
						t.Fatal(err)
					}
					want := ActionDeny
					if tool == "read_file" || tool == "write_file" && scenario != "sandboxed" || tool == "web_fetch" && scenario == "no-exec" {
						want = ActionAllow
					}
					// no-exec does not prohibit a recognized messaging tool by itself.
					if tool == "send_message" && scenario == "no-exec" {
						want = ActionAllow
					}
					if d.Action != want {
						t.Fatalf("want %s, got %s: %s", want, d.Action, d.Reason)
					}
				})
			}
		}
	}
}
