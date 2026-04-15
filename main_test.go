package main

import (
	"reflect"
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
