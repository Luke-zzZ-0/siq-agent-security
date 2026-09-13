package export

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"time"
	"unicode"
	"unicode/utf8"

	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/completion"
	"siq-agent-security/apps/agentshield/internal/effectevidence"
	"siq-agent-security/apps/agentshield/internal/intent"
	"siq-agent-security/apps/agentshield/internal/signing"
)

const TraceScope = "redacted_trace_projection_only"

var (
	traceHash   = regexp.MustCompile(`^[a-f0-9]{64}$`)
	traceRef    = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	traceReason = regexp.MustCompile(`^[a-z][a-z0-9_]{0,127}$`)
)

type TraceSourceInput struct {
	Seq         int
	ReceiptHash string
	Status      string
	Source      *intent.HistoricalSkillSource
}

type TraceInput struct {
	Activity         ActivityDocument
	Sources          []TraceSourceInput
	TaskID           string
	CompletionReason string
	Completion       *completion.Result
	Effects          []effectevidence.Record
	Now              time.Time
}

type TraceDocument struct {
	Schema        string              `json:"schema_version"`
	Scope         string              `json:"attestation_scope"`
	ActivityID    string              `json:"activity_id"`
	Snapshot      string              `json:"snapshot"`
	GeneratedAt   string              `json:"generated_at"`
	SourceCount   int                 `json:"source_count"`
	SourceTipHash string              `json:"source_tip_hash"`
	SourceLastSeq int                 `json:"source_last_seq"`
	PrefixValid   bool                `json:"prefix_valid"`
	History       string              `json:"history_integrity"`
	Incomplete    bool                `json:"incomplete"`
	PublicKey     string              `json:"public_key_base64"`
	Receipts      []ActivityExportRow `json:"receipts"`
	Sources       []TraceSourceRow    `json:"sources"`
	Completion    TraceCompletion     `json:"completion"`
	Effects       []TraceEffect       `json:"effects"`
	SigningSchema string              `json:"signing_schema"`
	Signature     string              `json:"signature"`
}

type TraceSourceRow struct {
	Seq         int               `json:"seq"`
	ReceiptHash string            `json:"receipt_hash"`
	Status      string            `json:"status"`
	Source      *TraceSkillSource `json:"source"`
}

type TraceSkillSource struct {
	GrantRef           string             `json:"grant_ref"`
	AdmissionRef       string             `json:"admission_ref"`
	PermissionDigest   string             `json:"permission_digest"`
	SkillNameRef       string             `json:"skill_name_ref"`
	DeclaredVersionRef *string            `json:"declared_version_ref"`
	ContentHash        string             `json:"content_hash"`
	Import             *TraceImportSource `json:"import"`
}

type TraceImportSource struct {
	ImportRef      string `json:"import_ref"`
	ArtifactDigest string `json:"artifact_digest"`
	AnalysisSHA256 string `json:"analysis_sha256"`
}

type TraceCompletion struct {
	Status       string             `json:"status"`
	ReasonCode   string             `json:"reason_code"`
	Requirements []TraceRequirement `json:"requirements"`
	IncidentRefs []string           `json:"incident_refs"`
}

type TraceRequirement struct {
	RequirementRef string   `json:"requirement_ref"`
	Status         string   `json:"status"`
	ReasonCode     string   `json:"reason_code"`
	EvidenceRefs   []string `json:"evidence_refs"`
}

type TraceEffect struct {
	EvidenceRef        string `json:"evidence_ref"`
	ActionRef          string `json:"action_ref"`
	DecisionReceiptRef string `json:"decision_receipt_ref"`
	EffectTypeRef      string `json:"effect_type_ref"`
	ResourceRef        string `json:"resource_ref"`
	ExecutionState     string `json:"execution_state"`
	SourceType         string `json:"source_type"`
	SourceRef          string `json:"source_ref"`
	Independence       string `json:"independence"`
	Coverage           string `json:"coverage"`
	Result             string `json:"result"`
	EvidenceDigest     string `json:"evidence_digest"`
	ObservedAt         string `json:"observed_at"`
	FindingCode        string `json:"finding_code"`
}

func safeTraceText(value string) bool {
	if value == "" || !utf8.ValidString(value) || len(value) > 1024 || utf8.RuneCountInString(value) > 256 {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func validActivityDocument(doc ActivityDocument, key *signing.Key) bool {
	if key == nil || VerifyActivity(key.Public(), doc) != nil || doc.PublicKey != key.PublicBase64() ||
		!traceHash.MatchString(doc.ActivityID) || !traceHash.MatchString(doc.Snapshot) ||
		doc.SourceCount < 1 || doc.SourceCount > 100000 || doc.SourceLastSeq < 0 ||
		!traceHash.MatchString(doc.SourceTipHash) || !doc.PrefixValid ||
		(doc.History != "verified" && doc.History != "unknown" && doc.History != "failed") ||
		len(doc.Receipts) < 1 || len(doc.Receipts) > 10000 {
		return false
	}
	previous := -1
	for _, row := range doc.Receipts {
		if row.Seq <= previous || !traceRef.MatchString(row.ReceiptRef) || !traceRef.MatchString(row.ToolRef) ||
			!traceHash.MatchString(row.SourceHash) ||
			(row.Action != "allow" && row.Action != "deny" && row.Action != "hold" && row.Action != "redact" && row.Action != "unknown") {
			return false
		}
		if row.IssuedAt != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, row.IssuedAt); err != nil || parsed.Format(time.RFC3339Nano) == "" {
				return false
			}
		}
		previous = row.Seq
	}
	return true
}

func projectTraceSource(input TraceSourceInput, receipt ActivityExportRow) (TraceSourceRow, bool) {
	row := TraceSourceRow{Seq: receipt.Seq, ReceiptHash: receipt.SourceHash, Status: input.Status}
	if input.Seq != receipt.Seq || input.ReceiptHash != receipt.SourceHash {
		return TraceSourceRow{}, false
	}
	if input.Status == "unavailable" {
		return row, input.Source == nil
	}
	if input.Status != "verified_source" || input.Source == nil {
		return TraceSourceRow{}, false
	}
	source := input.Source
	if !safeTraceText(source.Grant.GrantID) || !safeTraceText(source.Grant.AdmissionID) ||
		!traceHash.MatchString(source.Grant.PermissionDigest) || !safeTraceText(source.SkillName) ||
		!traceHash.MatchString(source.ContentHash) {
		return TraceSourceRow{}, false
	}
	projected := &TraceSkillSource{
		GrantRef:         activityRef(source.Grant.GrantID),
		AdmissionRef:     activityRef(source.Grant.AdmissionID),
		PermissionDigest: source.Grant.PermissionDigest,
		SkillNameRef:     activityRef(source.SkillName),
		ContentHash:      source.ContentHash,
	}
	if source.DeclaredVersion != nil {
		if !safeTraceText(*source.DeclaredVersion) {
			return TraceSourceRow{}, false
		}
		ref := activityRef(*source.DeclaredVersion)
		projected.DeclaredVersionRef = &ref
	}
	if source.Import != nil {
		if _, err := source.Import.Canonical(); err != nil {
			return TraceSourceRow{}, false
		}
		projected.Import = &TraceImportSource{ImportRef: activityRef(source.Import.ImportID), ArtifactDigest: source.Import.ArtifactDigest, AnalysisSHA256: source.Import.AnalysisSHA256}
	}
	row.Source = projected
	return row, true
}

func projectTraceCompletion(taskID, reason string, result *completion.Result) (TraceCompletion, map[string]bool, bool) {
	if result == nil {
		if !traceReason.MatchString(reason) {
			return TraceCompletion{}, nil, false
		}
		return TraceCompletion{Status: "unavailable", ReasonCode: reason, Requirements: []TraceRequirement{}, IncidentRefs: []string{}}, map[string]bool{}, true
	}
	if result.SchemaVersion != "completion-status/v1" || result.TaskID != taskID ||
		!memberTrace(result.Status, "verified", "incomplete", "conflicting", "unknown") || !traceReason.MatchString(result.ReasonCode) || len(result.Requirements) > 128 || len(result.IncidentIDs) > effectevidence.MaxRecords {
		return TraceCompletion{}, nil, false
	}
	out := TraceCompletion{Status: result.Status, ReasonCode: result.ReasonCode, Requirements: []TraceRequirement{}, IncidentRefs: []string{}}
	referenced := map[string]bool{}
	requirements := map[string]bool{}
	allVerified := len(result.Requirements) > 0
	for _, item := range result.Requirements {
		if !safeTraceText(item.RequirementID) || requirements[item.RequirementID] || !memberTrace(item.Status, "verified", "incomplete", "conflicting", "unknown") || !traceReason.MatchString(item.ReasonCode) || len(item.EvidenceIDs) > effectevidence.MaxRecords {
			return TraceCompletion{}, nil, false
		}
		requirements[item.RequirementID] = true
		allVerified = allVerified && item.Status == "verified" && len(item.EvidenceIDs) > 0
		projected := TraceRequirement{RequirementRef: activityRef(item.RequirementID), Status: item.Status, ReasonCode: item.ReasonCode, EvidenceRefs: []string{}}
		seen := map[string]bool{}
		for _, id := range item.EvidenceIDs {
			if !safeTraceText(id) || seen[id] {
				return TraceCompletion{}, nil, false
			}
			seen[id], referenced[id] = true, true
			projected.EvidenceRefs = append(projected.EvidenceRefs, activityRef(id))
		}
		out.Requirements = append(out.Requirements, projected)
	}
	incidents := map[string]bool{}
	for _, id := range result.IncidentIDs {
		if !safeTraceText(id) || incidents[id] {
			return TraceCompletion{}, nil, false
		}
		incidents[id], referenced[id] = true, true
		out.IncidentRefs = append(out.IncidentRefs, activityRef(id))
	}
	if result.Status == "verified" && (!allVerified || len(result.IncidentIDs) != 0) {
		return TraceCompletion{}, nil, false
	}
	return out, referenced, true
}

func memberTrace(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func projectTraceEffects(records []effectevidence.Record, referenced map[string]bool, taskID string, public []byte, now time.Time) ([]TraceEffect, bool) {
	if len(records) > effectevidence.MaxRecords {
		return nil, false
	}
	byID := map[string]effectevidence.Record{}
	for _, record := range records {
		id := record.Evidence.EvidenceID
		if record.TaskID != taskID || record.Verify(public, now) != nil || byID[id].Evidence.EvidenceID != "" {
			return nil, false
		}
		byID[id] = record
	}
	ids := make([]string, 0, len(referenced))
	for id := range referenced {
		if _, exists := byID[id]; !exists {
			return nil, false
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]TraceEffect, 0, len(ids))
	for _, id := range ids {
		record := byID[id]
		evidence := record.Evidence
		observed, _ := time.Parse(time.RFC3339Nano, evidence.ObservedAt)
		out = append(out, TraceEffect{
			EvidenceRef: activityRef(id), ActionRef: activityRef(evidence.ActionID), DecisionReceiptRef: activityRef(evidence.DecisionReceiptID),
			EffectTypeRef: activityRef(evidence.EffectType), ResourceRef: evidence.ResourceRef, ExecutionState: evidence.ExecutionState,
			SourceType: evidence.Source.Type, SourceRef: activityRef(evidence.Source.SourceID), Independence: evidence.Source.Independence,
			Coverage: evidence.Coverage, Result: evidence.Result, EvidenceDigest: evidence.EvidenceDigest,
			ObservedAt: observed.UTC().Format(time.RFC3339Nano), FindingCode: record.FindingCode,
		})
	}
	return out, true
}

func traceDocumentMap(doc TraceDocument) (map[string]any, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	value, err := canon.Decode(raw)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("trace export: invalid document")
	}
	delete(m, "signature")
	return m, nil
}

func BuildTrace(key *signing.Key, input TraceInput) (TraceDocument, error) {
	if !validActivityDocument(input.Activity, key) || input.Now.IsZero() || !safeTraceText(input.TaskID) || len(input.Sources) != len(input.Activity.Receipts) {
		return TraceDocument{}, errors.New("trace export: invalid input")
	}
	doc := TraceDocument{
		Schema: "local-task-trace-export/v1", Scope: TraceScope, ActivityID: input.Activity.ActivityID, Snapshot: input.Activity.Snapshot,
		GeneratedAt: input.Now.UTC().Format(time.RFC3339Nano), SourceCount: input.Activity.SourceCount, SourceTipHash: input.Activity.SourceTipHash,
		SourceLastSeq: input.Activity.SourceLastSeq, PrefixValid: true, History: input.Activity.History, PublicKey: key.PublicBase64(),
		Receipts: append([]ActivityExportRow(nil), input.Activity.Receipts...), Sources: []TraceSourceRow{}, Effects: []TraceEffect{}, SigningSchema: signing.SchemaLocalCanonicalV1,
	}
	for i, source := range input.Sources {
		row, ok := projectTraceSource(source, input.Activity.Receipts[i])
		if !ok {
			return TraceDocument{}, errors.New("trace export: invalid source")
		}
		doc.Sources = append(doc.Sources, row)
		doc.Incomplete = doc.Incomplete || row.Status != "verified_source"
	}
	completionOut, referenced, ok := projectTraceCompletion(input.TaskID, input.CompletionReason, input.Completion)
	if !ok {
		return TraceDocument{}, errors.New("trace export: invalid completion")
	}
	doc.Completion = completionOut
	doc.Incomplete = doc.Incomplete || completionOut.Status != "verified"
	doc.Effects, ok = projectTraceEffects(input.Effects, referenced, input.TaskID, key.Public(), input.Now)
	if !ok {
		return TraceDocument{}, errors.New("trace export: invalid effects")
	}
	m, err := traceDocumentMap(doc)
	if err != nil {
		return TraceDocument{}, err
	}
	doc.Signature, err = key.SignCanonical(m)
	return doc, err
}

func VerifyTrace(public []byte, doc TraceDocument) error {
	if doc.Schema != "local-task-trace-export/v1" || doc.Scope != TraceScope || doc.Signature == "" {
		return ErrTampered
	}
	m, err := traceDocumentMap(doc)
	if err != nil {
		return err
	}
	if signing.VerifyDocument(public, m, doc.Signature) != nil {
		return ErrTampered
	}
	return nil
}
