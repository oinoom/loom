package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseCommentReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{name: "numeric", raw: "2857259586", want: 2857259586},
		{name: "issue comment url", raw: "https://github.com/owner/repo/pull/24#issuecomment-2857259586", want: 2857259586},
		{name: "discussion url", raw: "https://github.com/owner/repo/pull/24#discussion_r2857259586", want: 2857259586},
		{name: "review comment endpoint url", raw: "https://api.github.com/repos/owner/repo/pulls/comments/2857259586", want: 2857259586},
		{name: "top level comment endpoint url", raw: "https://api.github.com/repos/owner/repo/issues/comments/2857259586", want: 2857259586},
		{name: "invalid", raw: "not-a-comment", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCommentReference(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFindThreadIDByComment(t *testing.T) {
	t.Parallel()

	records := []threadRecord{
		{
			ThreadID:  "PRRT_root",
			CommentID: 101,
			AllComments: []commentNode{
				{DatabaseID: 101},
				{DatabaseID: 102},
			},
		},
		{
			ThreadID:  "PRRT_reply",
			CommentID: 201,
			AllComments: []commentNode{
				{DatabaseID: 201},
				{DatabaseID: 202},
			},
		},
	}

	tests := []struct {
		commentID int64
		want      string
		wantErr   bool
	}{
		{commentID: 101, want: "PRRT_root"},
		{commentID: 202, want: "PRRT_reply"},
		{commentID: 999, wantErr: true},
	}

	for _, tt := range tests {
		got, err := findThreadIDByComment(records, tt.commentID)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for comment %d", tt.commentID)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for comment %d: %v", tt.commentID, err)
		}
		if got != tt.want {
			t.Fatalf("got %q, want %q", got, tt.want)
		}
	}
}

func TestResolveListFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		jsonFlag bool
		want     string
		wantErr  bool
	}{
		{name: "json alias wins", raw: "table", jsonFlag: true, want: "json"},
		{name: "table", raw: "table", want: "table"},
		{name: "json", raw: "json", want: "json"},
		{name: "jsonl", raw: "jsonl", want: "jsonl"},
		{name: "invalid", raw: "yaml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveListFormat(tt.raw, tt.jsonFlag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSortRecordsStableTieBreakers(t *testing.T) {
	t.Parallel()

	sameTime := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	records := []threadRecord{
		{ThreadID: "PRRT_c", CommentID: 3, UpdatedAt: sameTime, CreatedAt: sameTime, Path: "b.go", Line: 10, Author: "bob"},
		{ThreadID: "PRRT_a", CommentID: 1, UpdatedAt: sameTime, CreatedAt: sameTime, Path: "a.go", Line: 20, Author: "alice"},
		{ThreadID: "PRRT_b", CommentID: 2, UpdatedAt: sameTime, CreatedAt: sameTime, Path: "a.go", Line: 10, Author: "alice"},
	}

	sortRecords(records, "updated", true)

	got := []string{records[0].ThreadID, records[1].ThreadID, records[2].ThreadID}
	want := []string{"PRRT_b", "PRRT_a", "PRRT_c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got order %v, want %v", got, want)
	}
}

func TestNormalizeIssueCloseReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty", raw: "", want: ""},
		{name: "completed", raw: "completed", want: "completed"},
		{name: "not planned", raw: "not_planned", want: "not_planned"},
		{name: "invalid", raw: "done", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeIssueCloseReason(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	t.Parallel()

	got := splitCSV("bug, enhancement , , docs")
	want := []string{"bug", "enhancement", "docs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWantsCommandHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "dash help", args: []string{"--help"}, want: true},
		{name: "short help", args: []string{"-h"}, want: true},
		{name: "word help", args: []string{"help"}, want: true},
		{name: "body value is not help", args: []string{"--body", "help"}, want: false},
		{name: "no args", args: nil, want: false},
		{name: "multiple args", args: []string{"--help", "--json"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := wantsCommandHelp(tt.args); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandHelpText(t *testing.T) {
	t.Parallel()

	listText, ok := commandHelpText("list")
	if !ok {
		t.Fatal("expected list help")
	}
	if !strings.Contains(listText, "USAGE") || !strings.Contains(listText, "JSON FIELDS") {
		t.Fatalf("list help is missing expected sections:\n%s", listText)
	}

	aliasText, ok := commandHelpText("ls")
	if !ok {
		t.Fatal("expected ls help alias")
	}
	if aliasText != listText {
		t.Fatal("expected list and ls help text to match")
	}

	commentParentText, ok := commandHelpText("comment")
	if !ok {
		t.Fatal("expected comment help")
	}
	if !strings.Contains(commentParentText, "SUBCOMMANDS") || !strings.Contains(commentParentText, "comment top") {
		t.Fatalf("comment help is missing expected sections:\n%s", commentParentText)
	}

	commentTopText, ok := commandHelpText("comment top")
	if !ok {
		t.Fatal("expected comment top help")
	}
	commentTopAliasText, ok := commandHelpText("comment-top")
	if !ok {
		t.Fatal("expected comment-top help alias")
	}
	if commentTopAliasText != commentTopText {
		t.Fatal("expected comment top and comment-top help text to match")
	}

	editText, ok := commandHelpText("comment edit")
	if !ok {
		t.Fatal("expected comment edit help")
	}
	editAliasText, ok := commandHelpText("edit")
	if !ok {
		t.Fatal("expected edit help alias")
	}
	if editAliasText != editText {
		t.Fatal("expected comment edit and edit help text to match")
	}

	issueText, ok := commandHelpText("issue")
	if !ok {
		t.Fatal("expected issue help")
	}
	if !strings.Contains(issueText, "SUBCOMMANDS") || !strings.Contains(issueText, "issue create") {
		t.Fatalf("issue help is missing expected sections:\n%s", issueText)
	}

	issueCreateText, ok := commandHelpText("issue create")
	if !ok {
		t.Fatal("expected issue create help")
	}
	if !strings.Contains(issueCreateText, "--title") {
		t.Fatalf("issue create help is missing expected flags:\n%s", issueCreateText)
	}

	prText, ok := commandHelpText("pr")
	if !ok {
		t.Fatal("expected pr help")
	}
	if !strings.Contains(prText, "SUBCOMMANDS") || !strings.Contains(prText, "pr merge") {
		t.Fatalf("pr help is missing expected sections:\n%s", prText)
	}

	threadText, ok := commandHelpText("thread")
	if !ok {
		t.Fatal("expected thread help")
	}
	if !strings.Contains(threadText, "SUBCOMMANDS") || !strings.Contains(threadText, "thread resolve") {
		t.Fatalf("thread help is missing expected sections:\n%s", threadText)
	}

	resolveText, ok := commandHelpText("resolve")
	if !ok {
		t.Fatal("expected resolve help")
	}
	if !strings.Contains(resolveText, "--thread-id") || !strings.Contains(resolveText, "--comment-id") {
		t.Fatalf("resolve help is missing expected flags:\n%s", resolveText)
	}

	if _, ok := commandHelpText("unknown"); ok {
		t.Fatal("did not expect help for unknown command")
	}
}
