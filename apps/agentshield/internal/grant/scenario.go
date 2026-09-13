package grant

import (
	"errors"
	"fmt"

	"siq-agent-security/apps/agentshield/internal/admission"
)

// Scenario templates (UX-007): a closed catalog of named restriction presets
// applied at grant build time. A scenario can only DROP declared facts — it
// never adds permissions, so the resulting grant is always a strict subset of
// what the admission declared. The applied scenario identity is signed into
// the grant so approvals bind to the exact restriction shape.
var ErrScenarioInvalid = errors.New("grant_scenario_invalid")

// ScenarioRef is the signed identity of the applied scenario template.
type ScenarioRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// Scenario is a restriction preset: facts whose domain (or domain+action
// pair) is dropped are removed from the admission before the grant is built.
type Scenario struct {
	ID          string
	Version     int
	Name        string
	Description string
	DropDomains []string
	DropActions []string
}

// scenarios is the closed built-in catalog. Filesystem reads are implicit
// (never declared), so dropping fs.write plus exec/network domains yields a
// read-only shape. Tool and model facts are kept in every scenario: a
// read-only analysis still needs its declared tools and model access.
var scenarios = []Scenario{
	{
		ID:          "no-network",
		Version:     1,
		Name:        "无网络",
		Description: "移除全部网络声明（http.request / socket.connect），其余保持 admission 声明。",
		DropDomains: []string{"network"},
	},
	{
		ID:          "no-exec",
		Version:     1,
		Name:        "无执行",
		Description: "移除进程执行与包安装声明，其余保持 admission 声明。",
		DropDomains: []string{"process", "resource"},
	},
	{
		ID:          "sandboxed",
		Version:     1,
		Name:        "沙箱只读",
		Description: "移除网络、执行、包安装与文件写入声明；仅保留工具与模型声明和隐式只读访问。",
		DropDomains: []string{"network", "process", "resource"},
		DropActions: []string{"fs.write"},
	},
}

// ScenarioByID returns the built-in scenario with the given id, or nil.
func ScenarioByID(id string) *Scenario {
	for i := range scenarios {
		if scenarios[i].ID == id {
			sc := scenarios[i]
			return &sc
		}
	}
	return nil
}

// Scenarios returns the closed catalog (order fixed; no user-defined entries).
func Scenarios() []Scenario {
	out := make([]Scenario, len(scenarios))
	copy(out, scenarios)
	return out
}

// scenarioDropped reports whether a declared fact is excluded by the scenario.
func scenarioDropped(sc Scenario, d admission.DeclaredFact) bool {
	for _, domain := range sc.DropDomains {
		if d.Domain == domain {
			return true
		}
	}
	for _, action := range sc.DropActions {
		if d.Action == action {
			return true
		}
	}
	return false
}

// ApplyScenario filters an admission's declared facts down to the scenario
// shape. It is fail-closed on the scenario identity: an empty catalog hit is
// impossible (only built-ins are reachable), and filtering never widens.
func ApplyScenario(adm admission.Admission, sc Scenario) admission.Admission {
	kept := adm.DeclaredFacts[:0:0]
	for _, d := range adm.DeclaredFacts {
		if !scenarioDropped(sc, d) {
			kept = append(kept, d)
		}
	}
	adm.DeclaredFacts = kept
	return adm
}

// ResolveScenario validates an optional scenario id against the closed
// catalog, returning nil for "" (baseline build).
func ResolveScenario(id string) (*Scenario, error) {
	if id == "" {
		return nil, nil
	}
	sc := ScenarioByID(id)
	if sc == nil {
		return nil, fmt.Errorf("%w: unknown scenario %q", ErrScenarioInvalid, id)
	}
	return sc, nil
}
