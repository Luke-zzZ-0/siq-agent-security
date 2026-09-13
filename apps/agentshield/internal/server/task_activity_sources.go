package server

import (
	"net/http"
	"strings"

	"siq-agent-security/apps/agentshield/internal/importsource"
	"siq-agent-security/apps/agentshield/internal/intent"
)

type activitySkillSource struct {
	GrantID          string               `json:"grant_id"`
	AdmissionID      string               `json:"admission_id"`
	PermissionDigest string               `json:"permission_digest"`
	SkillName        string               `json:"skill_name"`
	Version          *string              `json:"declared_version"`
	ContentHash      string               `json:"content_hash"`
	Import           *importsource.Source `json:"import"`
}
type activitySourceRow struct {
	Seq    int                  `json:"seq"`
	Hash   string               `json:"receipt_hash"`
	Status string               `json:"status"`
	Source *activitySkillSource `json:"source"`
}
type activitySources struct {
	Schema   string              `json:"schema_version"`
	ID       string              `json:"activity_id"`
	Snapshot string              `json:"snapshot"`
	View     string              `json:"view"`
	Offset   int                 `json:"offset"`
	Total    int                 `json:"total"`
	Next     *int                `json:"next_offset"`
	Items    []activitySourceRow `json:"items"`
}

func (s *Server) taskActivitySources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/task-activities/"), "/sources")
	if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		w.WriteHeader(404)
		return
	}
	q, invalid := parseActivityQuery(r)
	if invalid || q.snapshot == "" {
		writeJSON(w, 400, map[string]string{"error": "task_activity_query_invalid"})
		return
	}
	all, projection, err := s.d.Engine.TaskActivitySnapshot()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
		return
	}
	meta := projectActivityPage(all, projection, q.view, 0, 0)
	if q.snapshot != meta.Snapshot {
		writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
		return
	}
	item, indexes, found := findTaskActivity(all, projection, q.view, id)
	if !found {
		writeJSON(w, 404, map[string]string{"error": "task_activity_not_found"})
		return
	}
	out := activitySources{Schema: "local-task-activity-sources/v1", ID: id, Snapshot: meta.Snapshot, View: q.view, Offset: q.offset, Total: len(indexes), Items: []activitySourceRow{}}
	cache := map[intent.HistoricalGrantSubject]*activitySkillSource{}
	end := min(q.offset+q.limit, len(indexes))
	for _, index := range indexes[min(q.offset, len(indexes)):end] {
		rc := all[index]
		row := activitySourceRow{Seq: rc.Seq, Hash: rc.Hash, Status: "unattributed"}
		if item.Binding != nil {
			row.Status = "unavailable"
			if rc.MatchedGrantID != nil && *rc.MatchedGrantID != "" && rc.AgentID != nil {
				subject := intent.HistoricalGrantSubject{Platform: rc.Platform, SessionID: rc.SessionID, AgentID: *rc.AgentID, TaskID: rc.TaskID, IntentID: rc.IntentID, IntentDigest: rc.IntentDigest, AuthorityRevision: rc.AuthorityRevision, MatchedGrantID: *rc.MatchedGrantID}
				value, exists := cache[subject]
				if !exists {
					if len(cache) >= 8 {
						writeJSON(w, 413, map[string]string{"error": "task_activity_sources_limit"})
						return
					}
					historical, err := s.intents.HistoricalSkillSource(subject, s.d.Store.GetHistoricalAdmission)
					if err == nil {
						value = &activitySkillSource{historical.Grant.GrantID, historical.Grant.AdmissionID, historical.Grant.PermissionDigest, historical.SkillName, historical.DeclaredVersion, historical.ContentHash, historical.Import}
					}
					cache[subject] = value
				}
				if value != nil {
					row.Status = "verified_source"
					row.Source = value
				}
			}
		}
		out.Items = append(out.Items, row)
	}
	if end < len(indexes) {
		out.Next = &end
	}
	latest, p, err := s.d.Engine.TaskActivitySnapshot()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
		return
	}
	if projectActivityPage(latest, p, q.view, 0, 0).Snapshot != meta.Snapshot {
		writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
		return
	}
	writeJSON(w, 200, out)
}
