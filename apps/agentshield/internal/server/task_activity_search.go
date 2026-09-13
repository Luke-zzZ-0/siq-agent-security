package server

import (
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"siq-agent-security/apps/agentshield/internal/receipt"
)

type activityFilters struct {
	Platform string `json:"platform"`
	Agent    string `json:"agent_id"`
	Session  string `json:"session_id"`
	Task     string `json:"task_id"`
	Query    string `json:"q"`
}
type activitySearch struct {
	taskActivityPage
	Filters activityFilters `json:"filters"`
}

func parseActivitySearch(r *http.Request) (activityQuery, activityFilters, bool) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	filters := activityFilters{}
	if err != nil {
		return activityQuery{}, filters, true
	}
	for name, target := range map[string]*string{"platform": &filters.Platform, "agent_id": &filters.Agent, "session_id": &filters.Session, "task_id": &filters.Task, "q": &filters.Query} {
		if vv, ok := values[name]; ok {
			if len(vv) != 1 || !utf8.ValidString(vv[0]) || len(vv[0]) > 1024 || utf8.RuneCountInString(vv[0]) > 256 {
				return activityQuery{}, filters, true
			}
			for _, char := range vv[0] {
				if unicode.IsControl(char) {
					return activityQuery{}, filters, true
				}
			}
			*target = vv[0]
			values.Del(name)
		}
	}
	clone := r.Clone(r.Context())
	u := *r.URL
	u.RawQuery = values.Encode()
	clone.URL = &u
	query, invalid := parseActivityQuery(clone)
	return query, filters, invalid
}
func (f activityFilters) matches(platform, agent, session, task string) bool {
	if f.Platform != "" && f.Platform != platform || f.Agent != "" && f.Agent != agent || f.Session != "" && f.Session != session || f.Task != "" && f.Task != task {
		return false
	}
	if f.Query == "" {
		return true
	}
	query := strings.ToLower(f.Query)
	for _, value := range []string{platform, agent, session, task} {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}
func filterActivities(all []receipt.Receipt, p receipt.TaskActivities, f activityFilters) receipt.TaskActivities {
	out := receipt.TaskActivities{Verification: p.Verification, Tasks: []receipt.TaskActivity{}, Unassigned: []int{}}
	for _, task := range p.Tasks {
		k := task.Key
		if f.matches(k.Platform, k.AgentID, k.SessionID, k.TaskID) {
			out.Tasks = append(out.Tasks, task)
		}
	}
	for _, index := range p.Unassigned {
		r := all[index]
		agent := ""
		if r.AgentID != nil {
			agent = *r.AgentID
		}
		if f.matches(r.Platform, agent, r.SessionID, r.TaskID) {
			out.Unassigned = append(out.Unassigned, index)
		}
	}
	return out
}
func (s *Server) taskActivitySearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	query, filters, invalid := parseActivitySearch(r)
	if invalid {
		writeJSON(w, 400, map[string]string{"error": "task_activity_query_invalid"})
		return
	}
	all, projection, err := s.d.Engine.TaskActivitySnapshot()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "task_activity_snapshot_unavailable"})
		return
	}
	page := projectActivityPage(all, filterActivities(all, projection, filters), query.view, query.offset, query.limit)
	if query.snapshot != "" && query.snapshot != page.Snapshot {
		writeJSON(w, 409, map[string]string{"error": "task_activity_snapshot_changed"})
		return
	}
	page.Schema = "local-task-activity-search/v1"
	writeJSON(w, 200, activitySearch{page, filters})
}
