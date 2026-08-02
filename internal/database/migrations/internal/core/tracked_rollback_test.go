package core

import (
	"strings"
	"testing"

	"go-build-admin/internal/conf"
)

func TestRollbackRejectsOfficialAndFrameworkTracks(t *testing.T) {
	for track, table := range map[string]string{"official": "migrations", "framework": "migrations_framework"} {
		_, err := RollbackTrackedMigrations(nil, &conf.Configuration{}, table, nil, RollbackOptions{TrackName: track})
		if err == nil || !strings.Contains(err.Error(), track+" migration rollback is unsupported") {
			t.Fatalf("track %s rollback error=%v", track, err)
		}
	}
}
