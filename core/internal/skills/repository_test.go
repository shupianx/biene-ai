package skills

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSetRepositoryDefaultEnabledByIDPersistsSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	root := createRepositoryTestSkill(t, "reviewer")
	other := createRepositoryTestSkill(t, "triage")

	cfg, err := SetRepositoryDefaultEnabledByID([]string{
		filepath.Base(root),
		filepath.Base(other),
	})
	if err != nil {
		t.Fatalf("SetRepositoryDefaultEnabledByID returned error: %v", err)
	}
	if len(cfg.DefaultEnabledSkillDirs) != 2 {
		t.Fatalf("expected 2 default-enabled skills, got %#v", cfg.DefaultEnabledSkillDirs)
	}
	if filepath.Clean(cfg.DefaultEnabledSkillDirs[0]) != filepath.Clean(root) {
		t.Fatalf("expected first dir %q, got %#v", root, cfg.DefaultEnabledSkillDirs)
	}
	if filepath.Clean(cfg.DefaultEnabledSkillDirs[1]) != filepath.Clean(other) {
		t.Fatalf("expected second dir %q, got %#v", other, cfg.DefaultEnabledSkillDirs)
	}
}

func TestDeleteRepositorySkillByIDRemovesSkillAndConfigEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	root := createRepositoryTestSkill(t, "reviewer")
	if _, err := SaveRepositoryConfig(RepositoryConfig{DefaultEnabledSkillDirs: []string{root}}); err != nil {
		t.Fatalf("SaveRepositoryConfig returned error: %v", err)
	}

	cfg, err := DeleteRepositorySkillByID("reviewer")
	if err != nil {
		t.Fatalf("DeleteRepositorySkillByID returned error: %v", err)
	}
	if len(cfg.DefaultEnabledSkillDirs) != 0 {
		t.Fatalf("expected empty config after delete, got %#v", cfg.DefaultEnabledSkillDirs)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("expected deleted skill dir to be removed, stat err=%v", err)
	}
}

func TestImportRepositoryFilesCopiesUploadedFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	srcDir := t.TempDir()
	skillRoot := filepath.Join(srcDir, "triage")
	if err := os.MkdirAll(filepath.Join(skillRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(`---
name: triage
description: Sort work
---
Use this skill.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "docs", "notes.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	importedID, err := ImportRepositoryFiles([]UploadedFile{
		{
			Path: "triage/SKILL.md",
			Open: func() (io.ReadCloser, error) {
				return os.Open(filepath.Join(skillRoot, "SKILL.md"))
			},
		},
		{
			Path: "triage/docs/notes.txt",
			Open: func() (io.ReadCloser, error) {
				return os.Open(filepath.Join(skillRoot, "docs", "notes.txt"))
			},
		},
	})
	if err != nil {
		t.Fatalf("ImportRepositoryFiles returned error: %v", err)
	}

	repositoryRoot, err := EnsureRepositoryRoot()
	if err != nil {
		t.Fatalf("EnsureRepositoryRoot returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, importedID, "SKILL.md")); err != nil {
		t.Fatalf("expected imported skill file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, importedID, "docs", "notes.txt")); err != nil {
		t.Fatalf("expected imported nested file: %v", err)
	}
}

func createRepositoryTestSkill(t *testing.T, name string) string {
	t.Helper()

	repositoryRoot, err := EnsureRepositoryRoot()
	if err != nil {
		t.Fatalf("EnsureRepositoryRoot returned error: %v", err)
	}

	dir := filepath.Join(repositoryRoot, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: ` + name + `
description: test skill
---
Use this skill.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
