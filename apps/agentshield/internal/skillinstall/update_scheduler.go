package skillinstall

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const updateScheduleRunSchema = "local-skill-update-schedule-run/v1"
const updateScheduleRunResultSchema = "local-skill-update-schedule-run-result/v1"

// ScheduleRunSchema is the request schema used to drive one scheduled check
// run; the daemon maintenance loop is the only non-HTTP caller.
const ScheduleRunSchema = updateScheduleRunSchema

// DaemonScheduledCheckActor attributes scheduler runs the daemon started on
// its own cadence, so audit records never borrow a human actor identity.
const DaemonScheduledCheckActor = "daemon-scheduled-check"

// Per-run and per-store bounds: a restart storm is absorbed by the run cap
// and the deterministic re-stagger, never by a burst of upstream requests.
const (
	defaultRunMaxChecks   = 4
	maxRunMaxChecks       = 16
	maxScheduleEntries    = 256
	runFailureBackoffBase = 15 * time.Minute
)

type ScheduledChecksRequest struct {
	SchemaVersion string `json:"schema_version"`
	ActorID       string `json:"actor_id"`
	MaxChecks     int    `json:"max_checks,omitempty"`
}

type ScheduledCheckOutcome struct {
	InstallID            string `json:"install_id"`
	Status               string `json:"status"`
	FailureCategory      string `json:"failure_category,omitempty"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	CheckedAt            string `json:"checked_at,omitempty"`
}

type ScheduledChecksResult struct {
	SchemaVersion string                  `json:"schema_version"`
	StartedAt     string                  `json:"started_at"`
	FinishedAt    string                  `json:"finished_at"`
	Total         int                     `json:"total"`
	Disabled      int                     `json:"disabled"`
	Due           int                     `json:"due"`
	NotDue        int                     `json:"not_due"`
	Stale         int                     `json:"stale"`
	Deferred      int                     `json:"deferred"`
	Checked       []ScheduledCheckOutcome `json:"checked"`
}

// backoffDelay doubles from the base with each consecutive failure, capped at
// the record's own interval so backoff never widens the user's cadence.
func backoffDelay(count int, interval time.Duration) time.Duration {
	if count < 1 {
		count = 1
	}
	delay := runFailureBackoffBase
	for i := 1; i < count && delay < interval; i++ {
		delay *= 2
	}
	if delay > interval {
		delay = interval
	}
	return delay
}

// restaggerDelay spreads a deferred install deterministically across its
// interval: the same install lands in the same slot after every restart, so
// the herd cannot re-form at startup.
func restaggerDelay(id string, interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}
	sum := sha256.Sum256([]byte("update-schedule-restagger:" + id))
	return time.Duration(binary.BigEndian.Uint64(sum[:8]) % uint64(interval))
}

func (s *Store) enumerateSchedules(ctx context.Context) ([]*UpdateSchedule, int, error) {
	dir := filepath.Join(s.dir, "update-sources")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, ErrUnavailable
	}
	if len(entries) > maxScheduleEntries {
		return nil, 0, ErrLimit
	}
	var out []*UpdateSchedule
	invalid := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !validInstallID(id) {
			// Foreign files in update-sources/ are never schedule metadata.
			invalid++
			continue
		}
		sched, err := s.readUpdateSchedule(ctx, id)
		if err != nil || sched == nil {
			// A record that fails its signature or invariant checks is
			// skipped, never repaired and never fetched against.
			invalid++
			continue
		}
		out = append(out, sched)
	}
	return out, invalid, nil
}

func (s *Store) RunScheduledChecks(ctx context.Context, req ScheduledChecksRequest) (*ScheduledChecksResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		// Fail fast: reading records with a dead context would misreport
		// entries as invalid rather than skip them.
		return nil, err
	}
	if req.SchemaVersion != updateScheduleRunSchema || !actorIDValid(req.ActorID) || req.MaxChecks < 0 || req.MaxChecks > maxRunMaxChecks {
		return nil, ErrInvalid
	}
	maxChecks := req.MaxChecks
	if maxChecks == 0 {
		maxChecks = defaultRunMaxChecks
	}
	started := s.now().UTC()
	now := started
	valid, invalid, err := s.enumerateSchedules(ctx)
	if err != nil {
		return nil, err
	}
	res := &ScheduledChecksResult{SchemaVersion: updateScheduleRunResultSchema, StartedAt: started.Format(time.RFC3339Nano), Checked: []ScheduledCheckOutcome{}, Total: len(valid) + invalid}
	var due []*UpdateSchedule
	for _, sched := range valid {
		if !sched.Enabled {
			res.Disabled++
			continue
		}
		next, err := time.Parse(time.RFC3339Nano, sched.NextCheckAt)
		if err != nil {
			// An enabled record without a usable due time is malformed.
			invalid++
			continue
		}
		if next.After(now) {
			continue
		}
		due = append(due, sched)
	}
	sort.Slice(due, func(i, j int) bool {
		if due[i].NextCheckAt != due[j].NextCheckAt {
			return due[i].NextCheckAt < due[j].NextCheckAt
		}
		return due[i].InstallID < due[j].InstallID
	})
	res.Stale = invalid
	res.Due = len(due)
	var deferred []*UpdateSchedule
	for _, sched := range due {
		if ctx.Err() != nil {
			deferred = append(deferred, sched)
			continue
		}
		if len(res.Checked) >= maxChecks {
			deferred = append(deferred, sched)
			continue
		}
		outcome, skipped := s.runScheduledCheck(ctx, sched, req.ActorID, now)
		switch skipped {
		case "":
			res.Checked = append(res.Checked, *outcome)
		case "canceled":
			deferred = append(deferred, sched)
		case "not_due":
			res.NotDue++
		default:
			res.Stale++
		}
	}
	// Re-stagger whatever this run could not take, so a restart that finds a
	// full due queue spreads it over the interval instead of retrying in
	// lockstep on the next run.
	for _, sched := range deferred {
		if ctx.Err() != nil {
			break
		}
		id := sched.InstallID
		interval := time.Duration(sched.IntervalSeconds) * time.Second
		if err := s.writeScheduleUpdate(ctx, id, func(m *UpdateSchedule, now time.Time) {
			m.NextCheckAt = now.Add(restaggerDelay(id, interval) + s.jitter(interval/8)).Format(time.RFC3339Nano)
		}, sched.Signature); err == nil {
			res.Deferred++
		}
	}
	res.FinishedAt = s.now().UTC().Format(time.RFC3339Nano)
	if err := ctx.Err(); err != nil {
		// A canceled run reports what it completed; nothing partial is written.
		return res, err
	}
	return res, nil
}

// runScheduledCheck performs one gated scheduled check. It re-reads the
// record after taking the stage gate, so a manual check that ran in the
// meantime wins: its recorded outcome pushed next_check_at out and this
// scheduled run becomes a no-op.
func (s *Store) runScheduledCheck(ctx context.Context, sched *UpdateSchedule, actor string, now time.Time) (*ScheduledCheckOutcome, string) {
	select {
	case stageSlot <- struct{}{}:
		defer func() { <-stageSlot }()
	case <-ctx.Done():
		return nil, "canceled"
	}
	id := sched.InstallID
	current, err := s.readUpdateSchedule(ctx, id)
	if err != nil || current == nil || !current.Enabled {
		return nil, "stale"
	}
	next, err := time.Parse(time.RFC3339Nano, current.NextCheckAt)
	if err != nil {
		return nil, "stale"
	}
	if next.After(now) {
		return nil, "not_due"
	}
	record, installed, err := s.historicalRecord(ctx, id)
	if err != nil || record.RecordedStatus != "installed_unverified" || installed == nil {
		return nil, "stale"
	}
	if s.removalStarted(id) != nil {
		return nil, "stale"
	}
	binding, err := installBindingDigest(record)
	if err != nil || binding != current.InstallBindingDigest {
		return nil, "stale"
	}
	result, err := s.checkUpdate(ctx, id, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", RemoteURL: current.Locator, ActorID: actor})
	interval := time.Duration(current.IntervalSeconds) * time.Second
	if err != nil {
		status, category, recordable := updateFailureOf(err)
		if !recordable {
			return nil, "stale"
		}
		count := current.FailureCount + 1
		if count > 63 {
			count = 63
		}
		backoff := backoffDelay(count, interval)
		if werr := s.writeScheduleUpdate(ctx, id, func(m *UpdateSchedule, now time.Time) {
			m.LastAttemptAt = now.Format(time.RFC3339Nano)
			m.LastStatus = status
			m.FailureCategory = category
			m.FailureCount = count
			m.NextCheckAt = now.Add(backoff + s.jitter(backoff/8)).Format(time.RFC3339Nano)
		}, current.Signature); werr != nil {
			return nil, "stale"
		}
		return &ScheduledCheckOutcome{InstallID: id, Status: status, FailureCategory: category}, ""
	}
	if werr := s.writeScheduleUpdate(ctx, id, func(m *UpdateSchedule, now time.Time) {
		m.LastAttemptAt = now.Format(time.RFC3339Nano)
		m.LastSuccessAt = now.Format(time.RFC3339Nano)
		m.LastStatus = result.Status
		m.FailureCategory = ""
		m.FailureCount = 0
		m.NextCheckAt = now.Add(interval + s.jitter(interval/8)).Format(time.RFC3339Nano)
	}, current.Signature); werr != nil {
		return nil, "stale"
	}
	return &ScheduledCheckOutcome{InstallID: id, Status: result.Status, RequiresConfirmation: result.RequiresConfirmation, CheckedAt: result.CheckedAt}, ""
}
