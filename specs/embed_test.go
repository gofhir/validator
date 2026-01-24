package specs

import (
	"testing"
)

func TestGetSpecsFS_R4(t *testing.T) {
	fs, dir, err := GetSpecsFS(R4)
	if err != nil {
		t.Fatalf("GetSpecsFS(R4) failed: %v", err)
	}

	if dir != "r4" {
		t.Errorf("expected dir 'r4', got '%s'", dir)
	}

	// Verify we can read profiles-resources.json
	data, err := fs.ReadFile("r4/profiles-resources.json")
	if err != nil {
		t.Fatalf("failed to read profiles-resources.json: %v", err)
	}

	if len(data) == 0 {
		t.Error("profiles-resources.json is empty")
	}

	t.Logf("profiles-resources.json size: %d bytes", len(data))
}

func TestGetSpecsFS_R4B(t *testing.T) {
	fs, dir, err := GetSpecsFS(R4B)
	if err != nil {
		t.Fatalf("GetSpecsFS(R4B) failed: %v", err)
	}

	if dir != "r4b" {
		t.Errorf("expected dir 'r4b', got '%s'", dir)
	}

	// Verify we can read profiles-resources.json
	data, err := fs.ReadFile("r4b/profiles-resources.json")
	if err != nil {
		t.Fatalf("failed to read profiles-resources.json: %v", err)
	}

	if len(data) == 0 {
		t.Error("profiles-resources.json is empty")
	}

	t.Logf("profiles-resources.json size: %d bytes", len(data))
}

func TestGetSpecsFS_R5(t *testing.T) {
	fs, dir, err := GetSpecsFS(R5)
	if err != nil {
		t.Fatalf("GetSpecsFS(R5) failed: %v", err)
	}

	if dir != "r5" {
		t.Errorf("expected dir 'r5', got '%s'", dir)
	}

	// Verify we can read profiles-resources.json
	data, err := fs.ReadFile("r5/profiles-resources.json")
	if err != nil {
		t.Fatalf("failed to read profiles-resources.json: %v", err)
	}

	if len(data) == 0 {
		t.Error("profiles-resources.json is empty")
	}

	t.Logf("profiles-resources.json size: %d bytes", len(data))
}

func TestListFiles(t *testing.T) {
	files, err := ListFiles(R4)
	if err != nil {
		t.Fatalf("ListFiles(R4) failed: %v", err)
	}

	if len(files) == 0 {
		t.Error("no files found for R4")
	}

	t.Logf("R4 files: %v", files)

	// Check that expected files exist
	expectedFiles := []string{
		"profiles-resources.json",
		"profiles-types.json",
		"v3-codesystems.json",
		"valuesets.json",
	}

	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	for _, expected := range expectedFiles {
		if !fileSet[expected] {
			t.Errorf("expected file %s not found", expected)
		}
	}
}

func TestReadFile(t *testing.T) {
	data, err := ReadFile(R4, "profiles-types.json")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("profiles-types.json is empty")
	}

	t.Logf("profiles-types.json size: %d bytes", len(data))
}

func TestHasFile(t *testing.T) {
	if !HasFile(R4, "profiles-resources.json") {
		t.Error("HasFile returned false for existing file")
	}

	if HasFile(R4, "nonexistent.json") {
		t.Error("HasFile returned true for nonexistent file")
	}
}

func TestV3CodeSystems(t *testing.T) {
	// R4 should have v3-codesystems.json
	if !HasFile(R4, "v3-codesystems.json") {
		t.Error("R4 should have v3-codesystems.json")
	}

	data, err := ReadFile(R4, "v3-codesystems.json")
	if err != nil {
		t.Fatalf("failed to read v3-codesystems.json: %v", err)
	}

	t.Logf("v3-codesystems.json size: %d bytes (%.2f MB)", len(data), float64(len(data))/1024/1024)
}
