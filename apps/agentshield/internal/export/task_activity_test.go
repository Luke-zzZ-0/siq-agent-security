package export

import (
	"bytes"
	"encoding/json"
	"testing"

	"siq-agent-security/apps/agentshield/internal/receipt"
	"siq-agent-security/apps/agentshield/internal/signing"
)

func TestActivityExportScopePrivacyAndIntegrity(t *testing.T) {
	key, err := signing.FromSeed(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	chain, err := receipt.OpenChain(t.TempDir(), "export", key)
	if err != nil {
		t.Fatal(err)
	}
	secret := "PRIVATE_SECRET_PARAMETER"
	agent := "agent"
	var all []receipt.Receipt
	for i := 0; i < 3; i++ {
		r := receipt.Receipt{ReceiptID: secret, Tool: secret, Reason: secret, ParamsExcerpt: &secret, Platform: "hermes", AgentID: &agent, SessionID: "session", TaskID: "task", IntentID: "intent", IntentDigest: "digest", IntentBinding: "bound", Action: "allow", IssuedAt: "2026-09-12T12:00:00+08:00"}
		if i == 1 {
			r.SessionID = "other"
		}
		if i == 2 {
			r.Action = secret
			r.IssuedAt = secret
		}
		if err := chain.Append(&r); err != nil {
			t.Fatal(err)
		}
		all = append(all, r)
	}
	binding := receipt.TaskActivityKey{ChainID: all[0].ChainID, Platform: "hermes", AgentID: agent, SessionID: "session", TaskID: "task", IntentID: "intent", IntentDigest: "digest"}
	got, err := ProjectActivity(all, key.Public(), nil, binding, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 2 || got.Rows[0].Seq != 0 || got.Rows[1].Seq != 2 || got.Rows[1].Action != "unknown" || got.Rows[1].IssuedAt != "" || got.Rows[0].IssuedAt != "2026-09-12T04:00:00Z" {
		t.Fatalf("unexpected projection: %+v", got)
	}
	if !got.SourceVerification.PrefixValid || got.SourceVerification.HistoryIntegrity == receipt.HistoryVerified {
		t.Fatal("incorrect integrity claim")
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatal("private text leaked")
	}
	for _, limit := range []int{0, 1, 100001} {
		if p, e := ProjectActivity(all, key.Public(), nil, binding, limit); e == nil || len(p.Rows) != 0 {
			t.Fatal("budget accepted")
		}
	}
	missing := binding
	missing.SessionID = "absent"
	if _, e := ProjectActivity(all, key.Public(), nil, missing, 2); e == nil {
		t.Fatal("missing scope accepted")
	}
	// Even tampering outside the requested activity invalidates its source snapshot.
	all[1].Reason = "tampered"
	if p, e := ProjectActivity(all, key.Public(), nil, binding, 2); e == nil || len(p.Rows) != 0 {
		t.Fatal("tampered source exported")
	}
}
