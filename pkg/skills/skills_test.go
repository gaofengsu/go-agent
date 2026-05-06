package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkill(t *testing.T) {
	content := `---
name: test-skill
description: A test skill
---

This is the body.`
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	skill, err := ParseSkill(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if skill.Name != "test-skill" {
		t.Errorf("name: got %q", skill.Name)
	}
	if skill.Description != "A test skill" {
		t.Errorf("description: got %q", skill.Description)
	}
	if skill.Body != "This is the body." {
		t.Errorf("body: got %q", skill.Body)
	}
}

func TestDiscoverSkills(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\ndescription: Test\n---\nBody"), 0644)

	store := NewStore(dir)
	skills, err := store.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("name: got %q", skills[0].Name)
	}
}

func TestActivateSkill(t *testing.T) {
	store := &Store{skills: []Skill{{Name: "test", Body: "Instructions here", Description: "Test"}}}
	result := store.Activate("test")
	if result == "" {
		t.Error("expected non-empty activation")
	}
	if result != "<activated_skill name=\"test\">\nInstructions here\n</activated_skill>" {
		t.Errorf("unexpected activation: %q", result)
	}
}

func TestActivateSkillNotFound(t *testing.T) {
	store := &Store{skills: []Skill{}}
	result := store.Activate("missing")
	if result == "" {
		t.Error("expected error message")
	}
}
