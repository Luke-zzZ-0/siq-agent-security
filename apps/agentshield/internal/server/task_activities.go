package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"siq-agent-security/apps/agentshield/internal/receipt"
)

type taskActivityItem struct {
	ID          string            `json:"activity_id"`
	Attribution string            `json:"attribution"`
	Binding     map[string]string `json:"binding"`
	Count       int               `json:"receipt_count"`
	First       int               `json:"first_seq"`
	Last        int               `json:"last_seq"`
}

type taskActivityPage struct {
	Schema      string             `json:"schema_version"`
	Snapshot    string             `json:"snapshot"`
	View        string             `json:"view"`
	Offset      int                `json:"offset"`
	Total       int                `json:"total"`
	Next        *int               `json:"next_offset"`
	PrefixValid bool               `json:"prefix_valid"`
	History     string             `json:"history_integrity"`
	Freshness   string             `json:"evidence_freshness"`
	Items       []taskActivityItem `json:"items"`
}

func activityDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func activityBinding(k receipt.TaskActivityKey) map[string]string {
	return map[string]string{"chain_id": k.ChainID, "platform": k.Platform, "session_id": k.SessionID, "agent_id": k.AgentID, "task_id": k.TaskID, "intent_id": k.IntentID, "intent_digest": k.IntentDigest}
}

func projectActivityPage(all []receipt.Receipt, projection receipt.TaskActivities, view string, offset, limit int) taskActivityPage {
	chain, tip := "", receipt.GenesisPrev
	if len(all) > 0 {
		chain, tip = all[0].ChainID, all[len(all)-1].Hash
	}
	page := taskActivityPage{Schema: "local-task-activities/v1", Snapshot: activityDigest([]any{chain, len(all), tip}), View: view, Offset: offset, PrefixValid: projection.Verification.PrefixValid, History: projection.Verification.HistoryIntegrity, Freshness: projection.Verification.EvidenceFreshness, Items: []taskActivityItem{}}
	page.Total = len(projection.Tasks)
	if view == "unassigned" {
		page.Total = len(projection.Unassigned)
	}
	end := offset + limit
	if end > page.Total {
		end = page.Total
	}
	for i := offset; i < end; i++ {
		if view == "unassigned" {
			r := all[projection.Unassigned[i]]
			page.Items = append(page.Items, taskActivityItem{ID: activityDigest([]any{r.ChainID, r.Seq, r.Hash}), Attribution: "unknown", Count: 1, First: r.Seq, Last: r.Seq})
		} else {
			task := projection.Tasks[i]
			binding := activityBinding(task.Key)
			indexes := task.ReceiptIndexes
			page.Items = append(page.Items, taskActivityItem{ID: activityDigest(binding), Attribution: "bound", Binding: binding, Count: len(indexes), First: all[indexes[0]].Seq, Last: all[indexes[len(indexes)-1]].Seq})
		}
	}
	if end < page.Total {
		page.Next = &end
	}
	return page
}

func (s *Server) taskActivities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	q, err := parseActivityQuery(r)
	if err {
		writeJSON(w, 400, map[string]string{"error": "task_activity_query_invalid"})
		return
	}
	all, projection, readErr := s.d.Engine.TaskActivitySnapshot()
	if readErr != nil {
		writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
		return
	}
	page := projectActivityPage(all, projection, q.view, q.offset, q.limit)
	if q.snapshot != "" && q.snapshot != page.Snapshot {
		writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
		return
	}
	writeJSON(w, 200, page)
}

type activityQuery struct {
	view, snapshot string
	offset, limit  int
}

func parseActivityQuery(r *http.Request) (activityQuery, bool) {
	out := activityQuery{view: "tasks", limit: 50}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return out, true
	}
	for name, v := range values {
		if len(v) != 1 {
			return out, true
		}
		switch name {
		case "view":
			out.view = v[0]
			if out.view != "tasks" && out.view != "unassigned" {
				return out, true
			}
		case "snapshot":
			out.snapshot = v[0]
			if len(out.snapshot) != 64 {
				return out, true
			}
			if _, err := hex.DecodeString(out.snapshot); err != nil {
				return out, true
			}
		case "offset", "limit":
			n, err := strconv.Atoi(v[0])
			if err != nil || strconv.Itoa(n) != v[0] || n < 0 || n > 100000 {
				return out, true
			}
			if name == "offset" {
				out.offset = n
			} else {
				if n < 1 || n > 100 {
					return out, true
				}
				out.limit = n
			}
		default:
			return out, true
		}
	}
	if out.offset > 0 && out.snapshot == "" {
		return out, true
	}
	return out, false
}
