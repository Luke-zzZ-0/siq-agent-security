package skillinstall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/canon"
	"siq-agent-security/apps/agentshield/internal/skillimport"
)

// schedulerFixture installs an upstream-capable skill, saves an enabled
// schedule for it and pins the store clock at a fixed instant that the test
// can advance through the returned pointer. Jitter is zeroed so due times are
// exact.
func schedulerFixture(t *testing.T) (fixture, *Operation, *time.Time) {
	t.Helper()
	f, op := installedInspection(t)
	s := f.store
	gitifyImportRecord(t, f)
	clock := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	s.jitter = func(time.Duration) time.Duration { return 0 }
	if _, err := s.SaveUpdateSource(context.Background(), op.InstallID, saveRequest("", true)); err != nil {
		t.Fatal(err)
	}
	return f, op, &clock
}

func runRequest(actor string) ScheduledChecksRequest {
	return ScheduledChecksRequest{SchemaVersion: updateScheduleRunSchema, ActorID: actor}
}

func scheduleDoc(t *testing.T, s *Store, id string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(s.updateSourcePath(id))
	if err != nil {
		t.Fatal(err)
	}
	value, err := canon.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := value.(map[string]any)
	if !ok {
		t.Fatal("schedule is not an object")
	}
	return doc
}

func snapInstall(t *testing.T, s *Store, id string) func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
	t.Helper()
	return func(ctx context.Context, rec *skillimport.Record, remoteURL string) (*skillimport.UpstreamSnapshot, error) {
		dirs, files := installedSnapshot(t, s, id)
		return &skillimport.UpstreamSnapshot{SourceKind: rec.SourceKind, CommitSHA: gitFixtureCommit, Directories: dirs, Files: files}, nil
	}
}

// TestScheduledCheckRunsDueAndRecordsOutcome covers the happy path: a due
// install is fetched, the outcome lands in schedule metadata only, the next
// due time moves out by the interval, and a manual check immediately after
// satisfies the current due slot so the scheduler has nothing left to do.
func TestScheduledCheckRunsDueAndRecordsOutcome(t *testing.T) {
	f, op, clock := schedulerFixture(t)
	s := f.store
	id := op.InstallID
	before := storeTree(t, s.dir)
	_, revisionBefore, err := s.authority.GetGrantWithSeq(f.request.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	s.upstream = snapInstall(t, s, id)
	*clock = clock.Add(25 * time.Hour)
	run, err := s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Due != 1 || len(run.Checked) != 1 || run.Disabled != 0 || run.NotDue != 0 || run.Stale != 0 {
		t.Fatalf("unexpected run %+v", run)
	}
	out := run.Checked[0]
	if out.InstallID != id || out.Status != "up_to_date" || out.RequiresConfirmation || out.CheckedAt != clock.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected outcome %+v", out)
	}
	view, err := s.ReadUpdateSchedule(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != UpdateStatusUpToDate || view.SourceState != UpdateSourceSaved || view.LastSuccessAt == "" {
		t.Fatalf("unexpected view %+v", view)
	}
	doc := scheduleDoc(t, s, id)
	if next, _ := doc["next_check_at"].(string); next != clock.Add(24*time.Hour).Format(time.RFC3339Nano) {
		t.Fatal("next due not interval-exact", next)
	}
	if count, ok := doc["failure_count"]; ok && count != float64(0) {
		t.Fatal("failure count not reset", count)
	}
	for _, key := range extraKeys(before, storeTree(t, s.dir)) {
		if !strings.HasPrefix(key, filepath.Join("update-sources", "")) {
			t.Fatal("scheduler wrote outside update-sources/", key)
		}
	}
	if _, revision, err := s.authority.GetGrantWithSeq(f.request.GrantID); err != nil || revision != revisionBefore {
		t.Fatal("scheduler touched authority", revision, revisionBefore)
	}
	// A manual check just after the run satisfies the current due slot: it
	// re-records the outcome and pushes next_check_at out, so the next run is
	// empty.
	*clock = clock.Add(2 * time.Hour)
	if _, err := s.CheckUpdate(context.Background(), id, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "human"}); err != nil {
		t.Fatal(err)
	}
	if next, _ := scheduleDoc(t, s, id)["next_check_at"].(string); next != clock.Add(24*time.Hour).Format(time.RFC3339Nano) {
		t.Fatal("manual check did not push next due", next)
	}
	run, err = s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Due != 0 || len(run.Checked) != 0 {
		t.Fatalf("scheduler re-ran a freshly checked install: %+v", run)
	}
}

// TestScheduledCheckNewVersionDoesNotConfirm verifies a scheduled check only
// flips schedule metadata to new_version and requires confirmation: install
// state and authority are untouched, so nothing is ever auto-confirmed.
func TestScheduledCheckNewVersionDoesNotConfirm(t *testing.T) {
	f, op, clock := schedulerFixture(t)
	s := f.store
	id := op.InstallID
	before := storeTree(t, s.dir)
	_, revisionBefore, err := s.authority.GetGrantWithSeq(f.request.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	s.upstream = func(ctx context.Context, rec *skillimport.Record, remoteURL string) (*skillimport.UpstreamSnapshot, error) {
		dirs, files := installedSnapshot(t, s, id)
		next := make([]skillimport.File, len(files), len(files)+2)
		copy(next, files)
		next = append(next,
			skillimport.File{Path: "CHANGE.md", SHA256: strings.Repeat("1", 64), Bytes: 8},
			skillimport.File{Path: "payload.sh", SHA256: strings.Repeat("2", 64), Bytes: 12, Executable: true},
		)
		return &skillimport.UpstreamSnapshot{SourceKind: rec.SourceKind, CommitSHA: strings.Repeat("f", 40), Directories: dirs, Files: next}, nil
	}
	*clock = clock.Add(25 * time.Hour)
	run, err := s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Due != 1 || len(run.Checked) != 1 || run.Checked[0].Status != "new_version" || !run.Checked[0].RequiresConfirmation {
		t.Fatalf("unexpected run %+v", run)
	}
	view, err := s.ReadUpdateSchedule(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != UpdateStatusNewVersion {
		t.Fatalf("unexpected view %+v", view)
	}
	for _, key := range extraKeys(before, storeTree(t, s.dir)) {
		if !strings.HasPrefix(key, filepath.Join("update-sources", "")) {
			t.Fatal("scheduler wrote outside update-sources/", key)
		}
	}
	if _, revision, err := s.authority.GetGrantWithSeq(f.request.GrantID); err != nil || revision != revisionBefore {
		t.Fatal("scheduler touched authority", revision, revisionBefore)
	}
}

// TestScheduledCheckFailureBackoff verifies consecutive failures back off
// geometrically from fifteen minutes while the last success timestamp is
// preserved and the status never reads "up to date".
func TestScheduledCheckFailureBackoff(t *testing.T) {
	f, op, clock := schedulerFixture(t)
	s := f.store
	id := op.InstallID
	s.upstream = func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
		return nil, skillimport.ErrSourceUnavailable
	}
	wants := []time.Duration{15 * time.Minute, 30 * time.Minute, time.Hour}
	*clock = clock.Add(25 * time.Hour)
	for i, want := range wants {
		run, err := s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
		if err != nil {
			t.Fatal(err)
		}
		if run.Due != 1 || len(run.Checked) != 1 || run.Checked[0].Status != UpdateStatusSourceUnavailable || run.Checked[0].FailureCategory != "source_unavailable" {
			t.Fatalf("run %d: %+v", i+1, run)
		}
		if next, _ := scheduleDoc(t, s, id)["next_check_at"].(string); next != clock.Add(want).Format(time.RFC3339Nano) {
			t.Fatalf("run %d: backoff %s, want %s", i+1, next, want)
		}
		view, err := s.ReadUpdateSchedule(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if view.Status != UpdateStatusSourceUnavailable || view.FailureCategory != "source_unavailable" || view.LastSuccessAt != "" {
			t.Fatalf("run %d view: %+v", i+1, view)
		}
		*clock = clock.Add(want + time.Minute)
	}
}

// TestScheduledCheckSkipsDisabledNotDueAndStale verifies the three skip
// buckets: disabled entries are counted and never fetched, entries a manual
// check pushed out are not due, and metadata bound to a different install
// object is stale rather than fetched against.
func TestScheduledCheckSkipsDisabledNotDueAndStale(t *testing.T) {
	f, op, clock := schedulerFixture(t)
	s := f.store
	id := op.InstallID
	fetched := false
	s.upstream = func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
		fetched = true
		return nil, ErrUnavailable
	}
	// Disabled: counted, never fetched, nothing written.
	if _, err := s.SaveUpdateSource(context.Background(), id, saveRequest("", false)); err != nil {
		t.Fatal(err)
	}
	before := storeTree(t, s.dir)
	run, err := s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Disabled != 1 || run.Due != 0 || len(run.Checked) != 0 || fetched {
		t.Fatalf("disabled entry not skipped: %+v fetched=%v", run, fetched)
	}
	if len(extraKeys(before, storeTree(t, s.dir))) != 0 {
		t.Fatal("disabled run wrote state")
	}
	// Re-enabled but not yet due: nothing happens.
	if _, err := s.SaveUpdateSource(context.Background(), id, saveRequest("", true)); err != nil {
		t.Fatal(err)
	}
	run, err = s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Due != 0 || len(run.Checked) != 0 || fetched {
		t.Fatalf("not-due entry fetched: %+v fetched=%v", run, fetched)
	}
	// Due, but the record binds to a different install object: stale, never
	// fetched, never repaired.
	*clock = clock.Add(25 * time.Hour)
	path := s.updateSourcePath(id)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := canon.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc := value.(map[string]any)
	doc["install_binding_digest"] = strings.Repeat("a", 64)
	delete(doc, "signature")
	signature, err := s.key.SignCanonical(doc)
	if err != nil {
		t.Fatal(err)
	}
	doc["signature"] = signature
	signed, err := canon.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, signed, 0600); err != nil {
		t.Fatal(err)
	}
	before = storeTree(t, s.dir)
	run, err = s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Stale != 1 || run.Due != 1 || len(run.Checked) != 0 || fetched {
		t.Fatalf("stale entry fetched or repaired: %+v fetched=%v", run, fetched)
	}
	if len(extraKeys(before, storeTree(t, s.dir))) != 0 {
		t.Fatal("stale run wrote state")
	}
}

// TestSchedulerRequestValidation covers request bounds and the pure scheduling
// helpers: the backoff table and the deterministic re-stagger.
func TestSchedulerRequestValidation(t *testing.T) {
	f, op, _ := schedulerFixture(t)
	s := f.store
	if _, err := s.RunScheduledChecks(context.Background(), ScheduledChecksRequest{SchemaVersion: "local-skill-update-schedule-run/v2", ActorID: "scheduler"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("bad schema accepted", err)
	}
	if _, err := s.RunScheduledChecks(context.Background(), runRequest("")); !errors.Is(err, ErrInvalid) {
		t.Fatal("empty actor accepted", err)
	}
	if _, err := s.RunScheduledChecks(context.Background(), ScheduledChecksRequest{SchemaVersion: updateScheduleRunSchema, ActorID: "scheduler", MaxChecks: -1}); !errors.Is(err, ErrInvalid) {
		t.Fatal("negative cap accepted", err)
	}
	if _, err := s.RunScheduledChecks(context.Background(), ScheduledChecksRequest{SchemaVersion: updateScheduleRunSchema, ActorID: "scheduler", MaxChecks: maxRunMaxChecks + 1}); !errors.Is(err, ErrInvalid) {
		t.Fatal("oversized cap accepted", err)
	}
	day := 24 * time.Hour
	hour := time.Hour
	for _, tc := range []struct {
		count    int
		interval time.Duration
		want     time.Duration
	}{
		{1, day, 15 * time.Minute},
		{2, day, 30 * time.Minute},
		{3, day, time.Hour},
		{7, day, 16 * time.Hour},
		{8, day, day},
		{1, hour, 15 * time.Minute},
		{3, hour, hour},
		{4, hour, hour},
	} {
		if got := backoffDelay(tc.count, tc.interval); got != tc.want {
			t.Fatalf("backoffDelay(%d, %s) = %s, want %s", tc.count, tc.interval, got, tc.want)
		}
	}
	if restaggerDelay(op.InstallID, 0) != 0 || restaggerDelay(op.InstallID, -time.Hour) != 0 {
		t.Fatal("restagger defined for non-positive interval")
	}
	a, b := restaggerDelay(op.InstallID, day), restaggerDelay(op.InstallID, day)
	if a != b {
		t.Fatal("restagger is not deterministic", a, b)
	}
	if a < 0 || a >= day {
		t.Fatal("restagger outside the interval", a)
	}
	if a == restaggerDelay("sin-"+strings.Repeat("b", 64), day) {
		t.Fatal("restagger does not spread distinct installs")
	}
}

// TestScheduledCheckZipDispatch verifies the scheduler passes the saved
// locator for zip sources, whose records do not retain their URL.
func TestScheduledCheckZipDispatch(t *testing.T) {
	f, op, clock := schedulerFixture(t)
	s := f.store
	id := op.InstallID
	zipifyImportRecord(t, f)
	// The git-bound locator no longer matches a zip import; re-save with the
	// zip URL so the digest binds again.
	if _, err := s.SaveUpdateSource(context.Background(), id, saveRequest(zipFixtureURL, true)); err != nil {
		t.Fatal(err)
	}
	s.upstream = func(ctx context.Context, rec *skillimport.Record, remoteURL string) (*skillimport.UpstreamSnapshot, error) {
		if rec.SourceKind != "https_zip" || remoteURL != zipFixtureURL {
			t.Error("wrong dispatch", rec.SourceKind, remoteURL)
			return nil, ErrInvalid
		}
		dirs, files := installedSnapshot(t, s, id)
		return &skillimport.UpstreamSnapshot{SourceKind: "https_zip", ArchiveSHA256: zipFixtureDigest, Directories: dirs, Files: files}, nil
	}
	*clock = clock.Add(25 * time.Hour)
	run, err := s.RunScheduledChecks(context.Background(), runRequest("scheduler"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Due != 1 || len(run.Checked) != 1 || run.Checked[0].Status != "up_to_date" {
		t.Fatalf("unexpected run %+v", run)
	}
	doc := scheduleDoc(t, s, id)
	if locator, _ := doc["locator"].(string); locator != zipFixtureURL {
		t.Fatal("locator not persisted", locator)
	}
}

// TestScheduledCheckCanceledContext verifies cancellation behavior: a run
// started on a dead context fails fast without fetching, and a run canceled
// mid-flight returns its partial result plus the context error and writes no
// attempt metadata.
func TestScheduledCheckCanceledContext(t *testing.T) {
	f, op, clock := schedulerFixture(t)
	s := f.store
	id := op.InstallID
	*clock = clock.Add(25 * time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if run, err := s.RunScheduledChecks(ctx, runRequest("scheduler")); !errors.Is(err, context.Canceled) || run != nil {
		t.Fatal("pre-canceled run did not fail fast", run, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	s.upstream = func(ctx context.Context, rec *skillimport.Record, remoteURL string) (*skillimport.UpstreamSnapshot, error) {
		cancel()
		dirs, files := installedSnapshot(t, s, id)
		return &skillimport.UpstreamSnapshot{SourceKind: rec.SourceKind, CommitSHA: gitFixtureCommit, Directories: dirs, Files: files}, nil
	}
	run, err := s.RunScheduledChecks(ctx, runRequest("scheduler"))
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation not reported", err)
	}
	if run == nil || run.Due != 1 || len(run.Checked) != 0 {
		t.Fatalf("unexpected partial result %+v", run)
	}
	if _, ok := scheduleDoc(t, s, id)["last_attempt_at"]; ok {
		t.Fatal("canceled run wrote attempt metadata")
	}
	if _, err := s.ReadUpdateSchedule(context.Background(), id); err != nil {
		t.Fatal(err)
	}
}
