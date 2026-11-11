package commands

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	helper "buildadmin-go/internal/pkg/crud_helper"
)

func unmanagedTestResults() []helper.ApplyTableResult {
	return []helper.ApplyTableResult{
		{
			Table: "ops_banner",
			Unmanaged: []helper.ApplyChange{
				{Field: "status", Type: "unmanaged", Reason: "database attributes are not modeled by CRUD spec", Unmanaged: []string{"db: enum ok"}},
				{Field: "title", Type: "unmanaged", Reason: "database attributes are not modeled by CRUD spec"},
				{Field: "idx_created_at", Type: "index-drift", Reason: "index definition differs"},
				{Field: "legacy_idx", Type: "unmanaged-index", Reason: "index exists in database but is not declared in spec (kept)"},
				{Field: "<column order>", Type: "column-order-drift", Reason: "physical column order differs from spec field order", Unmanaged: []string{"db: id, status, title", "spec: id, title, status"}},
				{Field: "legacy_note", Type: "column-attribute-drift", Reason: "unrecognized drift"},
			},
		},
		{
			Table: "ops_help",
			Unmanaged: []helper.ApplyChange{
				{Field: "extra", Type: "unmanaged", Reason: "database attributes are not modeled by CRUD spec"},
			},
		},
	}
}

func collectNotes(results []helper.ApplyTableResult, opts unmanagedNoteOptions) string {
	var output bytes.Buffer
	printUnmanagedNotes(func(format string, args ...any) {
		_, _ = fmt.Fprintf(&output, format, args...)
	}, results, opts)
	return output.String()
}

func TestPrintUnmanagedNotesAggregatesPerTable(t *testing.T) {
	got := collectNotes(unmanagedTestResults(), unmanagedNoteOptions{Prefix: "[database] "})
	want := strings.Join([]string{
		"[database] note ops_banner: 2 unmanaged column attributes; run migrate --plan for details",
		"[database] note ops_banner: 2 unmanaged indexes (idx_created_at, legacy_idx); run migrate --plan for details",
		"[database] note ops_banner: column order differs from spec field order; realign via a business migration",
		"[database] note ops_banner: 1 other unmanaged difference; run migrate --plan for details",
		"[database] note ops_help: 1 unmanaged column attribute; run migrate --plan for details",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("aggregated notes =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, "WARNING") {
		t.Fatalf("notes must never print WARNING: %q", got)
	}
}

func TestPrintUnmanagedNotesVerboseExpandsEachChange(t *testing.T) {
	got := collectNotes(unmanagedTestResults(), unmanagedNoteOptions{Prefix: "CRUD apply ", Verbose: true})
	want := strings.Join([]string{
		"CRUD apply note ops_banner.status: database attributes are not modeled by CRUD spec (db: enum ok)",
		"CRUD apply note ops_banner.title: database attributes are not modeled by CRUD spec",
		"CRUD apply note ops_banner.idx_created_at: index definition differs",
		"CRUD apply note ops_banner.legacy_idx: index exists in database but is not declared in spec (kept)",
		"CRUD apply note ops_banner.<column order>: physical column order differs from spec field order (db: id, status, title, spec: id, title, status)",
		"CRUD apply note ops_banner.legacy_note: unrecognized drift",
		"CRUD apply note ops_help.extra: database attributes are not modeled by CRUD spec",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("verbose notes =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, "WARNING") {
		t.Fatalf("notes must never print WARNING: %q", got)
	}
}
