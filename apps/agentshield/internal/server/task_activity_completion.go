package server

import (
	"net/http"
	"strings"
	"time"

	"siq-agent-security/apps/agentshield/internal/completion"
)

type activityCompletion struct {
	Schema      string             `json:"schema_version"`
	Snapshot    string             `json:"snapshot"`
	Activity    taskActivityItem   `json:"activity"`
	EvaluatedAt string             `json:"evaluated_at"`
	Reason      string             `json:"reason_code"`
	Result      *completion.Result `json:"result"`
}

func (s *Server) taskActivityCompletion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/task-activities/"), "/completion")
	if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		w.WriteHeader(404)
		return
	}
	q, invalid := parseActivityQuery(r)
	if invalid || r.URL.Query().Has("offset") || r.URL.Query().Has("limit") {
		writeJSON(w, 400, map[string]string{"error": "task_activity_query_invalid"})
		return
	}
	all, projection, err := s.d.Engine.TaskActivitySnapshot()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
		return
	}
	meta := projectActivityPage(all, projection, q.view, 0, 0)
	if q.snapshot != "" && q.snapshot != meta.Snapshot {
		writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
		return
	}
	activity, _, ok := findTaskActivity(all, projection, q.view, id)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "task_activity_not_found"})
		return
	}
	now := time.Now()
	out := activityCompletion{Schema: "local-task-activity-completion/v1", Snapshot: meta.Snapshot, Activity: activity, EvaluatedAt: now.UTC().Format(time.RFC3339Nano), Reason: "attribution_unknown"}
	if activity.Binding != nil {
		var failure string
		out.Reason, out.Result, _, failure = s.evaluateActivityCompletion(activity, now)
		if failure != "" {
			writeJSON(w, 500, map[string]string{"error": failure})
			return
		}
	}
	latest, latestProjection, err := s.d.Engine.TaskActivitySnapshot()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
		return
	}
	if projectActivityPage(latest, latestProjection, q.view, 0, 0).Snapshot != meta.Snapshot {
		writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
		return
	}
	writeJSON(w, 200, out)
}
