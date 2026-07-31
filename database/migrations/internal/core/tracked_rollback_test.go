package core

import (
	"strings"
	"testing"

	"go-build-admin/conf"
)

func TestRollbackRejectsOfficialAndLocalTracks(t *testing.T) {
	for _, track := range []string{"official", "local"} {
		_, err := RollbackTrackedMigrations(nil, &conf.Configuration{}, track+"_migrations", nil, RollbackOptions{TrackName: track})
		if err == nil || !strings.Contains(err.Error(), track+" migration rollback is unsupported") {
			t.Fatalf("track %s rollback error=%v", track, err)
		}
	}
}
