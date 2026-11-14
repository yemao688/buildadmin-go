package app_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "buildadmin-go"

type boundaryRule struct {
	code           string
	ownerPrefix    string
	bannedPrefixes []string
}

type whitelistEntry struct {
	File       string
	ImportPath string
	Rule       string
	Reason     string
	Stage      int
}

type boundaryViolation struct {
	File       string
	Line       int
	ImportPath string
	Rule       string
}

var boundaryRules = []boundaryRule{
	{code: "R1", ownerPrefix: "admin/", bannedPrefixes: []string{modulePath + "/internal/api"}},
	{code: "R2", ownerPrefix: "api/", bannedPrefixes: []string{modulePath + "/internal/admin"}},
	{code: "R3", ownerPrefix: "common/", bannedPrefixes: []string{modulePath + "/internal/admin", modulePath + "/internal/api"}},
	// Handlers never open persistence connections themselves: all DB access is
	// mediated by repositories/domain services via DI. (github.com/go-sql-driver/mysql
	// stays allowed for error-code inspection; the gorm driver is not.)
	{code: "R4", ownerPrefix: "admin/handler/", bannedPrefixes: []string{modulePath + "/internal/infra/db", "gorm.io/driver/mysql"}},
	{code: "R5", ownerPrefix: "api/handler/", bannedPrefixes: []string{modulePath + "/internal/infra/db", "gorm.io/driver/mysql"}},
	// Services are transport-free orchestration: no gin, no net/http, and no
	// direct persistence connections. Anything the flow needs from the request
	// (actor, client IP, tokens) arrives as plain parameters; DB access stays
	// behind the repository/domain-service boundary.
	{code: "R6", ownerPrefix: "admin/service/", bannedPrefixes: []string{"github.com/gin-gonic/gin", "net/http", modulePath + "/internal/infra/db"}},
	{code: "R7", ownerPrefix: "api/service/", bannedPrefixes: []string{"github.com/gin-gonic/gin", "net/http", modulePath + "/internal/infra/db"}},
	// pkg is technical infrastructure below every channel: it must not import a
	// business channel, common domain services, or the global middleware layer.
	{code: "R8", ownerPrefix: "pkg/", bannedPrefixes: []string{modulePath + "/internal/admin", modulePath + "/internal/api", modulePath + "/internal/common", modulePath + "/internal/middleware", modulePath + "/internal/crudgen"}},
	// common is shared domain code: gin is a transport concern and may only
	// appear in admin/api handlers/middleware, never in common.
	{code: "R9", ownerPrefix: "common/", bannedPrefixes: []string{"github.com/gin-gonic/gin"}},
	// crudgen is a development-time tool layer. It may read model/pkg but must
	// not depend on a runtime channel; metadata access goes through TableStore.
	{code: "R10", ownerPrefix: "crudgen/", bannedPrefixes: []string{modulePath + "/internal/admin", modulePath + "/internal/api", modulePath + "/internal/common", modulePath + "/internal/middleware"}},
}

var importBoundaryWhitelist = []whitelistEntry{}

func TestImportBoundary(t *testing.T) {
	t.Parallel()
	actual, err := collectImportBoundaryViolations(".")
	if err != nil {
		t.Fatal(err)
	}

	actualByKey := indexBoundaryViolations(actual)
	whitelistByKey := indexBoundaryWhitelist(importBoundaryWhitelist)

	newViolations := unwhitelistedBoundaryViolations(actualByKey, whitelistByKey)
	sortBoundaryViolations(newViolations)

	staleWhitelist := staleBoundaryWhitelist(whitelistByKey, actualByKey)
	sortStaleBoundaryWhitelist(staleWhitelist)

	if len(newViolations) == 0 && len(staleWhitelist) == 0 {
		return
	}

	t.Fatal(boundaryFailureReport(newViolations, staleWhitelist))
}

func indexBoundaryViolations(violations []boundaryViolation) map[string]boundaryViolation {
	byKey := make(map[string]boundaryViolation, len(violations))
	for _, v := range violations {
		byKey[boundaryKey(v.File, v.ImportPath)] = v
	}
	return byKey
}

func indexBoundaryWhitelist(entries []whitelistEntry) map[string]whitelistEntry {
	byKey := make(map[string]whitelistEntry, len(entries))
	for _, entry := range entries {
		byKey[boundaryKey(entry.File, entry.ImportPath)] = entry
	}
	return byKey
}

func unwhitelistedBoundaryViolations(actual map[string]boundaryViolation, whitelist map[string]whitelistEntry) []boundaryViolation {
	var newViolations []boundaryViolation
	for key, violation := range actual {
		if _, ok := whitelist[key]; !ok {
			newViolations = append(newViolations, violation)
		}
	}
	return newViolations
}

func staleBoundaryWhitelist(whitelist map[string]whitelistEntry, actual map[string]boundaryViolation) []whitelistEntry {
	var staleWhitelist []whitelistEntry
	for key, entry := range whitelist {
		if _, ok := actual[key]; !ok {
			staleWhitelist = append(staleWhitelist, entry)
		}
	}
	return staleWhitelist
}

func sortBoundaryViolations(violations []boundaryViolation) {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		if violations[i].ImportPath != violations[j].ImportPath {
			return violations[i].ImportPath < violations[j].ImportPath
		}
		return violations[i].Line < violations[j].Line
	})
}

func sortStaleBoundaryWhitelist(entries []whitelistEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].File != entries[j].File {
			return entries[i].File < entries[j].File
		}
		return entries[i].ImportPath < entries[j].ImportPath
	})
}

func boundaryFailureReport(newViolations []boundaryViolation, staleWhitelist []whitelistEntry) string {
	var b strings.Builder
	b.WriteString("import boundary check failed\n")
	if len(newViolations) > 0 {
		b.WriteString("new violations:\n")
		for _, violation := range newViolations {
			fmt.Fprintf(&b, "  %s %s:%d -> %s\n", violation.Rule, violation.File, violation.Line, violation.ImportPath)
		}
	}
	if len(staleWhitelist) > 0 {
		b.WriteString("stale whitelist entries:\n")
		for _, entry := range staleWhitelist {
			fmt.Fprintf(&b, "  %s %s -> %s (stage %d: %s)\n", entry.Rule, entry.File, entry.ImportPath, entry.Stage, entry.Reason)
		}
	}
	return b.String()
}

func collectImportBoundaryViolations(root string) ([]boundaryViolation, error) {
	fset := token.NewFileSet()
	var violations []boundaryViolation

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		ruleCode, ok := ruleForFile(rel)
		if !ok {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}

		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("unquote import in %s: %w", rel, err)
			}
			if !isForbiddenImport(rel, importPath) {
				continue
			}
			violations = append(violations, boundaryViolation{
				File:       rel,
				Line:       fset.Position(spec.Pos()).Line,
				ImportPath: importPath,
				Rule:       ruleCode,
			})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		if violations[i].ImportPath != violations[j].ImportPath {
			return violations[i].ImportPath < violations[j].ImportPath
		}
		return violations[i].Line < violations[j].Line
	})

	return violations, nil
}

func ruleForFile(rel string) (string, bool) {
	for _, rule := range boundaryRules {
		if strings.HasPrefix(rel, rule.ownerPrefix) {
			return rule.code, true
		}
	}
	return "", false
}

func isForbiddenImport(relFile, importPath string) bool {
	for _, rule := range boundaryRules {
		if !strings.HasPrefix(relFile, rule.ownerPrefix) {
			continue
		}
		for _, prefix := range rule.bannedPrefixes {
			if hasImportPrefix(importPath, prefix) {
				return true
			}
		}
	}
	return false
}

func hasImportPrefix(importPath, prefix string) bool {
	return importPath == prefix || strings.HasPrefix(importPath, prefix+"/")
}

func boundaryKey(file, importPath string) string {
	return file + "\x00" + importPath
}
