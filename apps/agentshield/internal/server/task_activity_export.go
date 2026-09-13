package server

import (
	"net/http"
	"strings"
	"time"

	exportpkg "siq-agent-security/apps/agentshield/internal/export"
)

func (s *Server) taskActivityExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/task-activities/"), "/export")
	if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		w.WriteHeader(404)
		return
	}
	q, invalid := parseActivityQuery(r)
	if invalid || q.snapshot == "" || q.view != "tasks" || r.URL.Query().Has("offset") || r.URL.Query().Has("limit") {
		writeJSON(w, 400, map[string]string{"error": "task_activity_query_invalid"})
		return
	}
	all, groups, err := s.d.Engine.TaskActivitySnapshot()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
		return
	}
	meta := projectActivityPage(all, groups, "tasks", 0, 0)
	if q.snapshot != meta.Snapshot {
		writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
		return
	}
	_, indexes, found := findTaskActivity(all, groups, "tasks", id)
	if !found {
		writeJSON(w, 404, map[string]string{"error": "task_activity_not_found"})
		return
	}
	if len(indexes) > 10000 {
		writeJSON(w, 413, map[string]string{"error": "task_activity_export_limit"})
		return
	}
	for _, group := range groups.Tasks {
		if activityDigest(activityBinding(group.Key)) != id {
			continue
		}
		projected, err := exportpkg.ProjectActivity(all, s.d.Key.Public(), nil, group.Key, 10000)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "task_activity_export_unavailable"})
			return
		}
		last := all[len(all)-1]
		doc := exportpkg.ActivityDocument{ActivityID: id, Snapshot: meta.Snapshot, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), SourceCount: len(all), SourceTipHash: last.Hash, SourceLastSeq: last.Seq, PrefixValid: groups.Verification.PrefixValid, History: groups.Verification.HistoryIntegrity, Receipts: []exportpkg.ActivityExportRow{}}
		for _, row := range projected.Rows {
			doc.Receipts = append(doc.Receipts, exportpkg.ActivityExportRow{Seq: row.Seq, IssuedAt: row.IssuedAt, ReceiptRef: row.ReceiptRef, ToolRef: row.ToolRef, Action: row.Action, SourceHash: row.SourceHash})
		}
		if err := exportpkg.SealActivity(s.d.Key, &doc); err != nil {
			writeJSON(w, 500, map[string]string{"error": "task_activity_export_unavailable"})
			return
		}
		latest, p, err := s.d.Engine.TaskActivitySnapshot()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
			return
		}
		if projectActivityPage(latest, p, "tasks", 0, 0).Snapshot != meta.Snapshot {
			writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="siq-activity-`+id+`.json"`)
		writeJSON(w, 200, doc)
		return
	}
	writeJSON(w, 404, map[string]string{"error": "task_activity_not_found"})
}
