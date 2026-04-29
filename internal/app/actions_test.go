package app

import (
	"context"
	"strings"
	"testing"
)

func TestIsOpenTargetURL(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   bool
	}{
		{name: "https", target: "https://github.com/0maru/gh-zen/pull/36", want: true},
		{name: "http", target: "http://example.test/issues/1", want: true},
		{name: "missing host", target: "https:///path", want: false},
		{name: "file URL", target: "file:///tmp/worktree", want: false},
		{name: "option-like value", target: "-psn_0_123", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOpenTargetURL(tc.target); got != tc.want {
				t.Fatalf("expected %q validation to be %v, got %v", tc.target, tc.want, got)
			}
		})
	}
}

func TestSystemActionRunner_OpenRejectsUnsafeTarget(t *testing.T) {
	err := (systemActionRunner{}).Open(context.Background(), "-psn_0_123")
	if err == nil {
		t.Fatal("expected unsafe open target to fail before launching")
	}
	if !strings.Contains(err.Error(), "unsupported URL") {
		t.Fatalf("expected unsupported URL error, got %v", err)
	}
}
