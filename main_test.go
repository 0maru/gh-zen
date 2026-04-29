package main

import (
	"testing"

	"github.com/0maru/gh-zen/internal/workbench"
)

func TestRepoRefFromFullName(t *testing.T) {
	got, ok := repoRefFromFullName("0maru/gh-zen")
	if !ok {
		t.Fatalf("expected repo ref to parse")
	}
	if want := (workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}); got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}

	if _, ok := repoRefFromFullName(""); ok {
		t.Fatalf("expected empty repo name to be rejected")
	}
}

func TestSameRepoFullName(t *testing.T) {
	if !sameRepoFullName("Owner/Repo", "owner/repo") {
		t.Fatalf("expected repo names to compare case-insensitively")
	}
	if sameRepoFullName("owner/other", "owner/repo") {
		t.Fatalf("expected different repo names not to match")
	}
}
