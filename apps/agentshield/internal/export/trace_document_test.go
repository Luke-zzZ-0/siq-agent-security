package export

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/completion"
	"siq-agent-security/apps/agentshield/internal/effectevidence"
	"siq-agent-security/apps/agentshield/internal/importsource"
	"siq-agent-security/apps/agentshield/internal/intent"
	"siq-agent-security/apps/agentshield/internal/runtimeaction"
	"siq-agent-security/apps/agentshield/internal/signing"
)

func traceFixture(t *testing.T) (*signing.Key, TraceInput) {
	t.Helper()
	key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	activity := ActivityDocument{
		ActivityID: strings.Repeat("1", 64), Snapshot: strings.Repeat("2", 64), GeneratedAt: "2026-09-12T00:00:00Z",
		SourceCount: 3, SourceTipHash: strings.Repeat("3", 64), SourceLastSeq: 2, PrefixValid: true, History: "unknown",
		Receipts: []ActivityExportRow{{Seq: 0, IssuedAt: "2026-09-08T01:00:00Z", ReceiptRef: activityRef("PRIVATE_RECEIPT"), ToolRef: activityRef("PRIVATE_TOOL"), Action: "allow", SourceHash: strings.Repeat("4", 64)}},
	}
	if err := SealActivity(key, &activity); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/contracts/effect-evidence.sample.json")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := effectevidence.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Signature = ""
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	store, err := effectevidence.NewStore(t.TempDir(), key)
	if err != nil {
		t.Fatal(err)
	}
	action := effectevidence.Action{
		ActionID: evidence.ActionID, DecisionReceiptID: evidence.DecisionReceiptID, TaskID: "task-1",
		IssuedAt: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), Authorized: true,
		Effects: []string{evidence.EffectType}, Resources: []runtimeaction.ResourceRef{{Domain: "filesystem", Digest: strings.Repeat("b", 64)}},
	}
	record, err := store.Submit(evidence, action, evidence.Source, now)
	if err != nil {
		t.Fatal(err)
	}
	version := "PRIVATE_VERSION_1.2.3"
	source := intent.HistoricalSkillSource{
		Grant:     intent.GrantReference{GrantID: "PRIVATE_GRANT", AdmissionID: "PRIVATE_ADMISSION", PermissionDigest: strings.Repeat("5", 64)},
		SkillName: "PRIVATE_SKILL_NAME", DeclaredVersion: &version, ContentHash: strings.Repeat("6", 64),
		Import: &importsource.Source{SchemaVersion: "local-skill-import-permission-source/v1", ImportID: "si-" + strings.Repeat("7", 32), ArtifactDigest: strings.Repeat("8", 64), AnalysisSHA256: strings.Repeat("9", 64)},
	}
	result := completion.Result{
		SchemaVersion: "completion-status/v1", TaskID: "task-1", Status: "unknown", ReasonCode: "effect_evidence_insufficient",
		Requirements: []completion.Item{{RequirementID: "PRIVATE_REQUIREMENT", Status: "unknown", ReasonCode: "effect_evidence_insufficient", EvidenceIDs: []string{record.Evidence.EvidenceID}}}, IncidentIDs: []string{},
	}
	return key, TraceInput{
		Activity: activity, Sources: []TraceSourceInput{{Seq: 0, ReceiptHash: strings.Repeat("4", 64), Status: "verified_source", Source: &source}},
		TaskID: "task-1", CompletionReason: "evaluated", Completion: &result, Effects: []effectevidence.Record{record}, Now: now,
	}
}

func TestTraceExportPrivacySignatureAndFixture(t *testing.T) {
	key, input := traceFixture(t)
	doc, err := BuildTrace(key, input)
	if err != nil || VerifyTrace(key.Public(), doc) != nil {
		t.Fatal("trace unavailable", err)
	}
	if !doc.Incomplete || len(doc.Sources) != 1 || doc.Sources[0].Source == nil || len(doc.Effects) != 1 || doc.Completion.Status != "unknown" {
		t.Fatalf("incorrect trace: %+v", doc)
	}
	if doc.Sources[0].Source.DeclaredVersionRef == nil || *doc.Sources[0].Source.DeclaredVersionRef != activityRef("PRIVATE_VERSION_1.2.3") ||
		doc.Effects[0].EvidenceRef != activityRef("eff-1") || doc.Completion.Requirements[0].EvidenceRefs[0] != activityRef("eff-1") {
		t.Fatal("references do not join")
	}
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE", "task-1", "eff-1", "action-1", "rcp-1", "observer-1"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("private source leaked: %s", secret)
		}
	}
	encoded = append(encoded, '\n')
	fixture := filepath.Join("..", "..", "testdata", "contracts", "local-task-trace-export.json")
	if os.Getenv("SIQ_UPDATE_ACTIVITY_FIXTURES") == "1" {
		if err := os.WriteFile(fixture, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil || !bytes.Equal(encoded, want) {
		t.Fatal("fixture differs", err)
	}
	tampered := doc
	tampered.Effects = append([]TraceEffect(nil), doc.Effects...)
	tampered.Effects[0].Result = "conflicting"
	if VerifyTrace(key.Public(), tampered) == nil {
		t.Fatal("tampered projection accepted")
	}
	other, _ := signing.FromSeed(bytes.Repeat([]byte{8}, 32))
	if VerifyTrace(other.Public(), doc) == nil {
		t.Fatal("untrusted signer accepted")
	}
}

func TestTraceExportRejectsBrokenRelationships(t *testing.T) {
	key, input := traceFixture(t)
	tests := map[string]func(*TraceInput){
		"source seq":       func(v *TraceInput) { v.Sources[0].Seq = 1 },
		"source hash":      func(v *TraceInput) { v.Sources[0].ReceiptHash = strings.Repeat("0", 64) },
		"source status":    func(v *TraceInput) { v.Sources[0].Status = "current" },
		"source control":   func(v *TraceInput) { v.Sources[0].Source.SkillName = "bad\nname" },
		"missing evidence": func(v *TraceInput) { v.Effects = nil },
		"duplicate effect": func(v *TraceInput) { v.Effects = append(v.Effects, v.Effects[0]) },
		"other task":       func(v *TraceInput) { v.TaskID = "other-task" },
		"wrong result task": func(v *TraceInput) {
			copy := *v.Completion
			copy.TaskID = "other-task"
			v.Completion = &copy
		},
		"duplicate result evidence": func(v *TraceInput) {
			copy := *v.Completion
			copy.Requirements = append([]completion.Item(nil), copy.Requirements...)
			copy.Requirements[0].EvidenceIDs = []string{"eff-1", "eff-1"}
			v.Completion = &copy
		},
		"false verified": func(v *TraceInput) {
			copy := *v.Completion
			copy.Status = "verified"
			v.Completion = &copy
		},
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := input
			candidate.Sources = append([]TraceSourceInput(nil), input.Sources...)
			candidate.Effects = append([]effectevidence.Record(nil), input.Effects...)
			source := *input.Sources[0].Source
			candidate.Sources[0].Source = &source
			edit(&candidate)
			if doc, err := BuildTrace(key, candidate); err == nil || doc.Signature != "" {
				t.Fatal("broken trace signed")
			}
		})
	}
}

func TestTraceExportPreservesExplicitMissingState(t *testing.T) {
	key, input := traceFixture(t)
	input.Sources = []TraceSourceInput{{Seq: 0, ReceiptHash: strings.Repeat("4", 64), Status: "unavailable"}}
	input.Completion, input.Effects, input.CompletionReason = nil, nil, "intent_missing"
	doc, err := BuildTrace(key, input)
	if err != nil || !doc.Incomplete || doc.Sources[0].Source != nil || doc.Completion.Status != "unavailable" || len(doc.Effects) != 0 || VerifyTrace(key.Public(), doc) != nil {
		t.Fatal("missing state was hidden", err)
	}
	input.Sources[0].Source = &intent.HistoricalSkillSource{}
	if _, err := BuildTrace(key, input); err == nil {
		t.Fatal("unavailable source carried data")
	}
}
