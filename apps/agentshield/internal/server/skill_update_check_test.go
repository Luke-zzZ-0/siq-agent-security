package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/skillinstall"
)

func TestSkillUpdateCheckHTTPGuardsLeaveStateUntouched(t *testing.T) {
	s, apply, grantID := appliedHTTPFixture(t, true)
	code, installed := call(t, s, "POST", "/v1/skill-installations/apply", token, apply)
	if code != 200 {
		t.Fatal(code, installed)
	}
	id := installed["install_id"].(string)
	route := "/v1/skill-installations/operations/" + id + "/update-check"
	req := skillinstall.UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "fixture-human"}
	for _, credential := range []string{"", token} {
		r := loopbackRequest("POST", route, req)
		if credential != "" {
			r.Header.Set("Authorization", "Bearer "+credential)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		want := 401
		if credential != "" {
			want = 403
		}
		if w.Code != want {
			t.Fatal("update check capability", w.Code)
		}
	}
	if code, _ := call(t, s, "GET", route, token, nil); code != 405 {
		t.Fatal("update check method", code)
	}
	raw, _ := json.Marshal(req)
	for _, bad := range []string{`{}`, strings.Replace(string(raw), `"schema_version":`, `"schema_version":null,"schema_version":`, 1), strings.TrimSuffix(string(raw), "}") + `,"confirm_check":true}`} {
		r := loopbackRequest("POST", route, bad)
		r.Header.Set("Authorization", "Bearer "+s.bootAdmin)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("update check strict body", w.Code, w.Body.String())
		}
	}
	// The fixture install came from a local directory import, which has no
	// fetchable upstream to check.
	code, out := call(t, s, "POST", route, token, req)
	if code != 400 || out["error"] != "skill_install_invalid" {
		t.Fatal("local source accepted", code, out)
	}
	code, out = call(t, s, "POST", "/v1/skill-installations/operations/sin-"+strings.Repeat("9", 64)+"/update-check", token, req)
	if code != 404 || out["error"] != "skill_install_not_found" {
		t.Fatal("unknown install mapping", code, out)
	}
	// The check must not have advanced any state.
	code, removal := call(t, s, "GET", "/v1/skill-installations/operations/"+id+"/removal", token, nil)
	if code != 200 || removal["status"] != "not_requested" {
		t.Fatal("check started removal", code, removal)
	}
	code, beforeGrant := call(t, s, "GET", "/v1/grants/"+grantID, token, nil)
	if code != 200 {
		t.Fatal(code, beforeGrant)
	}
	req = skillinstall.UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", RemoteURL: "https://git.example.com/org/skill.git", ActorID: "fixture-human"}
	if code, _ = call(t, s, "POST", route, token, req); code != 400 {
		t.Fatal("git URL accepted for a local source", code)
	}
	code, current := call(t, s, "GET", "/v1/grants/"+grantID, token, nil)
	if code != 200 || current["grant"].(map[string]any)["signature"] != beforeGrant["grant"].(map[string]any)["signature"] || current["state_revision"] != beforeGrant["state_revision"] {
		t.Fatal("check touched the grant", code, current)
	}
}

func TestUpdateCheckErrorResponses(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{skillinstall.ErrUnavailable, 503, "skill_install_unavailable"},
		{skillinstall.ErrUpdateURLBlocked, 400, "skill_update_url_blocked"},
		{skillinstall.ErrUpdateSourceUnavailable, 503, "skill_update_source_unavailable"},
		{skillinstall.ErrChanged, 409, "skill_install_changed"},
	} {
		w := httptest.NewRecorder()
		skillInstallError(w, tc.err)
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.status || body["error"] != tc.code || len(body) != 1 {
			t.Fatal(w.Code, body)
		}
	}
}
