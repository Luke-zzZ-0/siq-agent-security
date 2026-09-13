package skillinstall

import (
	"context"
	"errors"
	"testing"

	"siq-agent-security/apps/agentshield/internal/skillimport"
)

func TestReviewerUpdateCheckPreservesFetchAvailability(t *testing.T) {
	for _, cause := range []error{skillimport.ErrUnavailable, skillimport.ErrDownloadFailed} {
		t.Run(cause.Error(), func(t *testing.T) {
			f, op := installedInspection(t)
			gitifyImportRecord(t, f)
			f.store.upstream = func(context.Context, *skillimport.Record, string) (*skillimport.UpstreamSnapshot, error) {
				return nil, cause
			}
			_, err := f.store.CheckUpdate(context.Background(), op.InstallID, UpdateCheckRequest{SchemaVersion: "local-skill-update-check/v1", ActorID: "fixture-reviewer"})
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("fetch failure should remain unavailable, not report changed installation: got %v", err)
			}
		})
	}
}
