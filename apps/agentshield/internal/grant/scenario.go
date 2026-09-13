package grant

import (
	"errors"
	"fmt"

	"siq-agent-security/apps/agentshield/internal/admission"
	"siq-agent-security/apps/agentshield/internal/runtimeaction"
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

// Scenarios restrict both declared capabilities and executable tool effects.
// Resource authorization still applies; none of these presets is OS isolation.
var scenarios = []Scenario{
	{
		ID:          "no-network",
		Version:     1,
		Name:        "无网络",
		Description: "禁止网络请求和消息发送，同时禁用无法保证不出网的通用执行与未知工具。",
		DropDomains: []string{"network", "process", "resource"},
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
		Name:        "只读权限",
		Description: "仅允许已识别的只读工具；禁止执行、写入、删除、出网和未知操作，不代表 OS 沙箱。",
		DropDomains: []string{"network", "process", "resource"},
		DropActions: []string{"fs.write"},
	},
}

// ScenarioByID returns the built-in scenario with the given id, or nil.
func ScenarioByID(id string) *Scenario {
	for i := range scenarios {
		if scenarios[i].ID == id {
			sc := scenarios[i]
			sc.DropDomains = append([]string(nil), sc.DropDomains...)
			sc.DropActions = append([]string(nil), sc.DropActions...)
			return &sc
		}
	}
	return nil
}

// Scenarios returns the closed catalog (order fixed; no user-defined entries).
func Scenarios() []Scenario {
	out := make([]Scenario, len(scenarios))
	for i := range scenarios {
		out[i] = *ScenarioByID(scenarios[i].ID)
	}
	return out
}

// scenarioDropped reports whether a declared fact is excluded by the scenario.
func scenarioDropped(sc Scenario, d admission.DeclaredFact) bool {
	if d.Domain == "tool" && !scenarioToolAllowed(&ScenarioRef{ID: sc.ID, Version: sc.Version}, d.Resource.Value) {
		return true
	}
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

// ScenarioAllowsEffects applies signed scenario restrictions to normalized
// runtime effects, including grants created before projection filtering existed.
// Unknown effects cannot prove compliance with a restrictive scenario.
func ScenarioAllowsEffects(ref *ScenarioRef, effects []string) bool {
	if ref == nil {
		return true
	}
	sc := ScenarioByID(ref.ID)
	if sc == nil || sc.Version != ref.Version || len(effects) == 0 {
		return false
	}
	for _, effect := range effects {
		switch effect {
		case runtimeaction.EffectToolInvoke, runtimeaction.EffectFileRead, runtimeaction.EffectFileWrite,
			runtimeaction.EffectFileDelete, runtimeaction.EffectNetworkRequest, runtimeaction.EffectProcessExec,
			runtimeaction.EffectMessageSend, runtimeaction.EffectDatabaseRead, runtimeaction.EffectDatabaseWrite,
			runtimeaction.EffectSecretRead:
		default:
			return false
		}
		switch ref.ID {
		case "no-exec":
			if effect == runtimeaction.EffectProcessExec {
				return false
			}
		case "no-network":
			if effect == runtimeaction.EffectNetworkRequest || effect == runtimeaction.EffectMessageSend || effect == runtimeaction.EffectProcessExec {
				return false
			}
		case "sandboxed":
			if effect != runtimeaction.EffectFileRead && effect != runtimeaction.EffectDatabaseRead && effect != runtimeaction.EffectToolInvoke {
				return false
			}
		}
	}
	return true
}

func scenarioToolAllowed(ref *ScenarioRef, tool string) bool {
	if ref == nil {
		return true
	}
	_, effects := runtimeaction.Normalize(tool, nil)
	return ScenarioAllowsEffects(ref, effects)
}

func restrictScenarioTools(ref *ScenarioRef, tools []string) []string {
	if ref == nil {
		return tools
	}
	kept := make([]string, 0, len(tools))
	for _, tool := range tools {
		if scenarioToolAllowed(ref, tool) {
			kept = append(kept, tool)
		}
	}
	return kept
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
