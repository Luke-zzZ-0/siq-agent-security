package grant

import (
	"strings"
	"testing"
)

// scenarioFacts returns the "domain|action" pairs kept in a built grant.
func scenarioFacts(res *Result) map[string]bool {
	out := map[string]bool{}
	for _, f := range res.Grant.Facts {
		out[f.Domain+"|"+f.Action] = true
	}
	return out
}

func buildScenario(t *testing.T, id string) *Result {
	t.Helper()
	sc := ScenarioByID(id)
	if sc == nil {
		t.Fatalf("unknown scenario %q", id)
	}
	res, err := Build(sampleAdmission(), Options{Subject: Subject{Type: "agent_instance", ID: "inst_1"},
		Platform: "hermes", Now: fixedNow, Key: key(t), Scenario: sc})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestScenarioNoNetworkDropsNetworkFacts(t *testing.T) {
	baseline := build(t, "hermes", sampleAdmission())
	res := buildScenario(t, "no-network")
	facts := scenarioFacts(res)
	if facts["network|http.request"] || facts["network|socket.connect"] {
		t.Fatalf("network facts survived no-network scenario: %v", facts)
	}
	base := scenarioFacts(baseline)
	for pair := range facts {
		if !base[pair] {
			t.Fatalf("scenario introduced fact %v not declared by admission", pair)
		}
	}
	for _, pair := range []string{"tool|tool.invoke", "model|model.generate"} {
		if !facts[pair] {
			t.Fatalf("scenario dropped unrelated fact %v", pair)
		}
	}
	if res.Grant.Scenario == nil || res.Grant.Scenario.ID != "no-network" || res.Grant.Scenario.Version != 1 {
		t.Fatalf("scenario ref missing or wrong: %+v", res.Grant.Scenario)
	}
	if !Verify(key(t).Public(), res.Grant) {
		t.Fatal("scenario grant signature invalid")
	}
	// tool allowlist projections stay consistent with the retained tool facts
	if res.Grant.HermesToolsetAllowlist == nil || len(*res.Grant.HermesToolsetAllowlist) == 0 {
		t.Fatal("hermes toolset allowlist lost by scenario")
	}
}

func TestScenarioNoExecDropsProcessAndResource(t *testing.T) {
	res := buildScenario(t, "no-exec")
	facts := scenarioFacts(res)
	if facts["process|process.exec"] {
		t.Fatal("process.exec survived no-exec scenario")
	}
	if facts["resource|package.install"] {
		t.Fatal("package.install survived no-exec scenario")
	}
	if !facts["network|http.request"] {
		t.Fatal("no-exec must keep network declarations")
	}
	if res.Grant.OpenClawToolPolicy != nil {
		for _, list := range [][]string{res.Grant.OpenClawToolPolicy.RequireApproval} {
			for _, tool := range list {
				if tool == "exec" {
					t.Fatal("openclaw exec gate survived no-exec scenario")
				}
			}
		}
	}
}

func TestScenarioSandboxedIsReadOnlyShape(t *testing.T) {
	res := buildScenario(t, "sandboxed")
	facts := scenarioFacts(res)
	for _, pair := range []string{"network|http.request", "process|process.exec", "resource|package.install", "filesystem|fs.write"} {
		if facts[pair] {
			t.Fatalf("sandboxed scenario kept %v", pair)
		}
	}
	if !facts["tool|tool.invoke"] || !facts["model|model.generate"] {
		t.Fatal("sandboxed scenario dropped tool or model declarations")
	}
}

func TestScenarioNeverWidensBeyondAdmission(t *testing.T) {
	baseline := build(t, "hermes", sampleAdmission())
	base := scenarioFacts(baseline)
	for _, id := range []string{"no-network", "no-exec", "sandboxed"} {
		facts := scenarioFacts(buildScenario(t, id))
		for pair := range facts {
			if !base[pair] {
				t.Fatalf("scenario %s introduced undeclared fact %v", id, pair)
			}
		}
		if len(facts) > len(base) {
			t.Fatalf("scenario %s produced more fact kinds than baseline", id)
		}
	}
}

func TestUnknownScenarioRejected(t *testing.T) {
	if ScenarioByID("total-access") != nil {
		t.Fatal("closed catalog leaked unknown scenario")
	}
	if _, err := ResolveScenario("total-access"); err == nil || !strings.Contains(err.Error(), "unknown scenario") {
		t.Fatalf("resolveScenario accepted unknown id: %v", err)
	}
	_, err := Build(sampleAdmission(), Options{Subject: Subject{Type: "agent_instance", ID: "inst_1"},
		Platform: "hermes", Now: fixedNow, Key: key(t),
		Scenario: &Scenario{ID: "total-access", Version: 1, DropDomains: []string{"tool"}}})
	if err == nil {
		t.Fatal("Build accepted a non-catalog scenario: catalog must stay closed")
	}
}

func TestScenarioCatalogIsClosedAndFailClosed(t *testing.T) {
	catalog := Scenarios()
	if len(catalog) != 3 {
		t.Fatalf("unexpected catalog size %d", len(catalog))
	}
	ids := map[string]bool{}
	for _, sc := range catalog {
		ids[sc.ID] = true
		if sc.Version < 1 || sc.Name == "" || sc.Description == "" {
			t.Fatalf("scenario %s missing identity fields", sc.ID)
		}
	}
	for _, want := range []string{"no-network", "no-exec", "sandboxed"} {
		if !ids[want] {
			t.Fatalf("catalog missing %s", want)
		}
	}
	// every catalog entry must be a pure restriction: dropping everything must
	// still leave DefaultEffect deny, and each entry only removes
	for _, sc := range catalog {
		res, err := Build(sampleAdmission(), Options{Subject: Subject{Type: "agent_instance", ID: "inst_1"},
			Platform: "hermes", Now: fixedNow, Key: key(t), Scenario: &sc})
		if err != nil {
			t.Fatalf("scenario %s failed to build: %v", sc.ID, err)
		}
		if res.Grant.DefaultEffect != "deny" {
			t.Fatalf("scenario %s changed default effect", sc.ID)
		}
	}
}

func TestDraftFromPreservesScenario(t *testing.T) {
	res := buildScenario(t, "sandboxed")
	approved := *res
	var err error
	approved.Grant, err = Approve(res.Grant, human(), key(t))
	if err != nil {
		t.Fatal(err)
	}
	draft, err := DraftFrom(approved.Grant, approved.DesiredPolicy, "grt-d-"+strings.Repeat("a", 64), fixedNow, key(t))
	if err != nil {
		t.Fatal(err)
	}
	if draft.Grant.Scenario == nil || draft.Grant.Scenario.ID != "sandboxed" {
		t.Fatal("draft lost the scenario binding")
	}
	if !Verify(key(t).Public(), draft.Grant) {
		t.Fatal("draft scenario grant signature invalid")
	}
}
