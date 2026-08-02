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

const modulePath = "go-build-admin"

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
}

var importBoundaryWhitelist = []whitelistEntry{}

func TestImportBoundary(t *testing.T) {
	actual, err := collectImportBoundaryViolations(".")
	if err != nil {
		t.Fatal(err)
	}

	actualByKey := make(map[string]boundaryViolation, len(actual))
	for _, v := range actual {
		actualByKey[boundaryKey(v.File, v.ImportPath)] = v
	}

	whitelistByKey := make(map[string]whitelistEntry, len(importBoundaryWhitelist))
	for _, entry := range importBoundaryWhitelist {
		whitelistByKey[boundaryKey(entry.File, entry.ImportPath)] = entry
	}

	var newViolations []boundaryViolation
	for key, violation := range actualByKey {
		if _, ok := whitelistByKey[key]; !ok {
			newViolations = append(newViolations, violation)
		}
	}

	var staleWhitelist []whitelistEntry
	for key, entry := range whitelistByKey {
		if _, ok := actualByKey[key]; !ok {
			staleWhitelist = append(staleWhitelist, entry)
		}
	}

	sort.Slice(newViolations, func(i, j int) bool {
		if newViolations[i].File != newViolations[j].File {
			return newViolations[i].File < newViolations[j].File
		}
		if newViolations[i].ImportPath != newViolations[j].ImportPath {
			return newViolations[i].ImportPath < newViolations[j].ImportPath
		}
		return newViolations[i].Line < newViolations[j].Line
	})
	sort.Slice(staleWhitelist, func(i, j int) bool {
		if staleWhitelist[i].File != staleWhitelist[j].File {
			return staleWhitelist[i].File < staleWhitelist[j].File
		}
		return staleWhitelist[i].ImportPath < staleWhitelist[j].ImportPath
	})

	if len(newViolations) == 0 && len(staleWhitelist) == 0 {
		return
	}

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

	t.Fatal(b.String())
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
