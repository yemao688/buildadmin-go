package crud_helper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomSkeletonFirstGenerationWritesModelAndHandler(t *testing.T) {
	root := t.TempDir()
	entityFile := NameInfo{
		ParseFile: filepath.Join(root, "internal", "model", "banner.go"),
		Namespace: "model",
		LastName:  "Banner",
	}
	repositoryFile := NameInfo{
		ParseFile: filepath.Join(root, "internal", "admin", "repository", "banner.go"),
		Namespace: "repository",
		LastName:  "Banner",
	}
	handlerFile := NameInfo{
		ParseFile: filepath.Join(root, "internal", "admin", "handler", "banner.go"),
		Namespace: "handler",
		LastName:  "Banner",
	}

	if err := writeCustomSkeleton(entityFile, "model"); err != nil {
		t.Fatal(err)
	}
	if err := writeCustomSkeleton(repositoryFile, "model"); err != nil {
		t.Fatal(err)
	}
	if err := writeCustomSkeleton(handlerFile, "handler"); err != nil {
		t.Fatal(err)
	}

	modelContent, err := os.ReadFile(customSkeletonPath(entityFile))
	if err != nil {
		t.Fatal(err)
	}
	repositoryContent, err := os.ReadFile(customSkeletonPath(repositoryFile))
	if err != nil {
		t.Fatal(err)
	}
	handlerContent, err := os.ReadFile(customSkeletonPath(handlerFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(modelContent) != customSkeletonContent("model", "Banner", "model") {
		t.Fatalf("model skeleton differs from template:\n%s", modelContent)
	}
	if string(repositoryContent) != customSkeletonContent("repository", "Banner", "model") {
		t.Fatalf("repository skeleton differs from template:\n%s", repositoryContent)
	}
	if string(handlerContent) != customSkeletonContent("handler", "Banner", "handler") {
		t.Fatalf("handler skeleton differs from template:\n%s", handlerContent)
	}
	if !strings.Contains(string(modelContent), "CRUD regeneration never overwrites") || !strings.Contains(string(modelContent), "// func (m *Banner) CustomHook()") {
		t.Fatalf("model skeleton is missing purpose or example comments:\n%s", modelContent)
	}
}

func TestCustomSkeletonSecondGenerationPreservesModifiedContent(t *testing.T) {
	file := NameInfo{
		ParseFile: filepath.Join(t.TempDir(), "banner.go"),
		Namespace: "model",
		LastName:  "Banner",
	}
	if err := writeCustomSkeleton(file, "model"); err != nil {
		t.Fatal(err)
	}
	customPath := customSkeletonPath(file)
	modified := "package model\n\n// business customization\n"
	if err := os.WriteFile(customPath, []byte(modified), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeCustomSkeleton(file, "model"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != modified {
		t.Fatalf("modified custom skeleton was overwritten:\n%s", content)
	}
}

func TestCustomSkeletonDeleteRemovesUnmodifiedFiles(t *testing.T) {
	root := t.TempDir()
	entityFile := NameInfo{ParseFile: filepath.Join(root, "internal", "model", "banner.go"), Namespace: "model", LastName: "Banner"}
	handlerFile := NameInfo{ParseFile: filepath.Join(root, "internal", "admin", "handler", "banner.go"), Namespace: "handler", LastName: "Banner"}
	for _, file := range []struct {
		info NameInfo
		kind string
	}{
		{entityFile, "model"},
		{handlerFile, "handler"},
	} {
		if err := writeCustomSkeleton(file.info, file.kind); err != nil {
			t.Fatal(err)
		}
	}

	paths, preserved, err := splitCustomSkeletonManifest([]string{customSkeletonPath(entityFile), customSkeletonPath(handlerFile)})
	if err != nil {
		t.Fatal(err)
	}
	if len(preserved) != 0 || len(paths) != 2 {
		t.Fatalf("unmodified custom skeleton split = generated %v, preserved %v", paths, preserved)
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{customSkeletonPath(entityFile), customSkeletonPath(handlerFile)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unmodified custom skeleton still exists at %s", path)
		}
	}
}

func TestCustomSkeletonDeletePreservesModifiedFilesWithWarning(t *testing.T) {
	file := NameInfo{ParseFile: filepath.Join(t.TempDir(), "internal", "model", "banner.go"), Namespace: "model", LastName: "Banner"}
	if err := writeCustomSkeleton(file, "model"); err != nil {
		t.Fatal(err)
	}
	path := customSkeletonPath(file)
	if err := os.WriteFile(path, []byte(customSkeletonContent("model", "Banner", "model")+"// customized\n"), 0644); err != nil {
		t.Fatal(err)
	}

	generated, preserved, err := splitCustomSkeletonManifest([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(generated) != 0 || len(preserved) != 1 || preserved[0] != path {
		t.Fatalf("modified custom skeleton split = generated %v, preserved %v", generated, preserved)
	}
	warning := customSkeletonWarning(preserved)
	if !strings.Contains(warning, "WARNING") || !strings.Contains(warning, filepath.ToSlash(path)) {
		t.Fatalf("warning = %q, want warning with preserved path", warning)
	}
}
