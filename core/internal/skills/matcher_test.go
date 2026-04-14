package skills

import "testing"

func TestSelectForTextHonorsExplicitReference(t *testing.T) {
	metas := []Metadata{
		{Name: "pr-description", Description: "Write polished pull request descriptions"},
		{Name: "reviewer", Description: "Review code carefully"},
	}

	match := SelectForText("请用 $pr-description 帮我写一下这个 PR 的描述", metas)
	if match == nil {
		t.Fatal("expected a skill match")
	}
	if match.Name != "pr-description" {
		t.Fatalf("expected pr-description, got %q", match.Name)
	}
}

func TestSelectForTextUsesBestLexicalMatch(t *testing.T) {
	metas := []Metadata{
		{Name: "pr-description", Description: "Write polished pull request descriptions for code changes"},
		{Name: "reviewer", Description: "Review code carefully and focus on correctness"},
	}

	match := SelectForText("please help me draft a pull request description for this change", metas)
	if match == nil {
		t.Fatal("expected a skill match")
	}
	if match.Name != "pr-description" {
		t.Fatalf("expected pr-description, got %q", match.Name)
	}
}

func TestSelectForTextReturnsNilForWeakMatches(t *testing.T) {
	metas := []Metadata{
		{Name: "release-notes", Description: "Generate release notes from commits"},
	}

	match := SelectForText("what time is it in berlin", metas)
	if match != nil {
		t.Fatalf("expected no match, got %q", match.Name)
	}
}
