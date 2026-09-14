package skillinstall

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestUpdateScheduleViewContractSample pins ReadUpdateSchedule's HTTP view
// against the committed contract sample that the web client validates
// against, so neither side can drift from the wire format.
func TestUpdateScheduleViewContractSample(t *testing.T) {
	f, op := installedInspection(t)
	gitifyImportRecord(t, f)
	f.store.now = func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) }
	if _, err := f.store.SaveUpdateSource(context.Background(), op.InstallID, saveRequest("", true)); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.ReadUpdateSchedule(context.Background(), op.InstallID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/contracts/local-skill-update-schedule-view.json")
	if err != nil {
		t.Fatal(err)
	}
	var want UpdateScheduleView
	if err = json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	want.InstallID = op.InstallID
	if !reflect.DeepEqual(got, &want) {
		t.Fatalf("view differs from contract sample: %#v", got)
	}
	// The view shown to the browser must not carry scheduling internals.
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"locator", "signature", "saved_by", "interval", "install_binding"} {
		if strings.Contains(string(encoded), leak) {
			t.Fatal("view leaks", leak)
		}
	}
}
