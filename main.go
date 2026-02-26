package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

const usage = `loom - sift and action GitHub PR review comments quickly

Usage:
  loom list --pr <number> [flags]
  loom find --pr <number> --query <text> [flags]
  loom reply --pr <number> --comment <id> (--body <text> | --body-file <file>)
  loom resolve --thread <node-id>
  loom unresolve --thread <node-id>

Common flags:
  --repo <owner/name>   Repository (default: current git remote)

Examples:
  loom list --repo ryuvel/tacara --pr 24
  loom list --pr 24 --state unresolved --severity major --sort created --desc
  loom find --pr 24 --query "stale rows" --path tacara-indexer/src/main.rs
  loom reply --pr 24 --comment 2857259586 --body "Addressed in <commit-url>"
  loom resolve --thread PRRT_kwDORR607s5w3N_2
`

const listThreadsQuery = `
query($owner:String!, $repo:String!, $number:Int!, $after:String) {
  repository(owner:$owner, name:$repo) {
    pullRequest(number:$number) {
      reviewThreads(first:100, after:$after) {
        nodes {
          id
          isResolved
          isOutdated
          path
          line
          originalLine
          comments(first:100) {
            nodes {
              id
              databaseId
              body
              url
              createdAt
              author { login }
              replyTo { databaseId }
            }
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}
`

const resolveThreadMutation = `
mutation($threadId:ID!) {
  resolveReviewThread(input:{threadId:$threadId}) {
    thread { id isResolved }
  }
}
`

const unresolveThreadMutation = `
mutation($threadId:ID!) {
  unresolveReviewThread(input:{threadId:$threadId}) {
    thread { id isResolved }
  }
}
`

type gqlThreadsResponse struct {
	Repository struct {
		PullRequest struct {
			ReviewThreads struct {
				Nodes    []threadNode `json:"nodes"`
				PageInfo struct {
					HasNextPage bool    `json:"hasNextPage"`
					EndCursor   *string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"reviewThreads"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

type threadNode struct {
	ID           string `json:"id"`
	IsResolved   bool   `json:"isResolved"`
	IsOutdated   bool   `json:"isOutdated"`
	Path         string `json:"path"`
	Line         *int   `json:"line"`
	OriginalLine *int   `json:"originalLine"`
	Comments     struct {
		Nodes []commentNode `json:"nodes"`
	} `json:"comments"`
}

type commentNode struct {
	ID         string `json:"id"`
	DatabaseID int64  `json:"databaseId"`
	Body       string `json:"body"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	Author     struct {
		Login string `json:"login"`
	} `json:"author"`
	ReplyTo *struct {
		DatabaseID int64 `json:"databaseId"`
	} `json:"replyTo"`
}

type threadRecord struct {
	ThreadID    string        `json:"thread_id"`
	CommentID   int64         `json:"comment_id"`
	Resolved    bool          `json:"resolved"`
	Outdated    bool          `json:"outdated"`
	Path        string        `json:"path"`
	Line        int           `json:"line"`
	Author      string        `json:"author"`
	Summary     string        `json:"summary"`
	Severity    string        `json:"severity"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	URL         string        `json:"url"`
	CommentBody string        `json:"comment_body"`
	AllComments []commentNode `json:"-"`
}

type listOptions struct {
	Repo     string
	PR       int
	State    string
	PathLike string
	Author   string
	Contains string
	Severity string
	SortBy   string
	Desc     bool
	Limit    int
	JSON     bool
	Stats    bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "list", "ls":
		err = runList(args, "")
	case "find":
		err = runFind(args)
	case "reply":
		err = runReply(args)
	case "resolve":
		err = runResolve(args, false)
	case "unresolve":
		err = runResolve(args, true)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runFind(args []string) error {
	fs := flag.NewFlagSet("find", flag.ContinueOnError)
	var opts listOptions
	var query string
	fs.StringVar(&opts.Repo, "repo", "", "owner/name repository")
	fs.IntVar(&opts.PR, "pr", 0, "pull request number")
	fs.StringVar(&query, "query", "", "text to search in review comments")
	fs.StringVar(&opts.PathLike, "path", "", "filter by file path substring")
	fs.StringVar(&opts.Author, "author", "", "filter by root comment author")
	fs.StringVar(&opts.State, "state", "unresolved", "unresolved|resolved|all")
	fs.StringVar(&opts.Severity, "severity", "", "critical|major|minor")
	fs.StringVar(&opts.SortBy, "sort", "updated", "updated|created|path|line|author|severity")
	fs.BoolVar(&opts.Desc, "desc", true, "descending sort")
	fs.IntVar(&opts.Limit, "limit", 200, "max rows to print")
	fs.BoolVar(&opts.JSON, "json", false, "output JSON")
	fs.BoolVar(&opts.Stats, "stats", false, "print grouped summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(query) == "" {
		return errors.New("--query is required")
	}
	opts.Contains = query
	return executeList(opts)
}

func runList(args []string, presetContains string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	var opts listOptions
	fs.StringVar(&opts.Repo, "repo", "", "owner/name repository")
	fs.IntVar(&opts.PR, "pr", 0, "pull request number")
	fs.StringVar(&opts.State, "state", "unresolved", "unresolved|resolved|all")
	fs.StringVar(&opts.PathLike, "path", "", "filter by file path substring")
	fs.StringVar(&opts.Author, "author", "", "filter by root comment author")
	fs.StringVar(&opts.Contains, "contains", presetContains, "filter by body substring")
	fs.StringVar(&opts.Severity, "severity", "", "critical|major|minor")
	fs.StringVar(&opts.SortBy, "sort", "updated", "updated|created|path|line|author|severity")
	fs.BoolVar(&opts.Desc, "desc", true, "descending sort")
	fs.IntVar(&opts.Limit, "limit", 200, "max rows to print")
	fs.BoolVar(&opts.JSON, "json", false, "output JSON")
	fs.BoolVar(&opts.Stats, "stats", false, "print grouped summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return executeList(opts)
}

func executeList(opts listOptions) error {
	if opts.PR <= 0 {
		return errors.New("--pr is required")
	}

	owner, repo, err := resolveRepo(opts.Repo)
	if err != nil {
		return err
	}

	records, err := fetchReviewThreads(owner, repo, opts.PR)
	if err != nil {
		return err
	}

	records = filterRecords(records, opts)
	sortRecords(records, opts.SortBy, opts.Desc)
	if opts.Limit > 0 && len(records) > opts.Limit {
		records = records[:opts.Limit]
	}

	if opts.Stats {
		printStats(records)
	}
	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}
	printTable(records)
	return nil
}

func runReply(args []string) error {
	fs := flag.NewFlagSet("reply", flag.ContinueOnError)
	var repoArg string
	var pr int
	var commentID int64
	var body string
	var bodyFile string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.IntVar(&pr, "pr", 0, "pull request number")
	fs.Int64Var(&commentID, "comment", 0, "pull request review comment database ID")
	fs.StringVar(&body, "body", "", "reply text")
	fs.StringVar(&bodyFile, "body-file", "", "read reply text from file")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if pr <= 0 {
		return errors.New("--pr is required")
	}
	if commentID <= 0 {
		return errors.New("--comment is required")
	}
	text, err := resolveBodyText(body, bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("reply body is empty")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/comments/%d/replies", owner, repo, pr, commentID)
	req := map[string]string{"body": text}
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	var out struct {
		ID      int64  `json:"id"`
		HTMLURL string `json:"html_url"`
	}
	if err := client.Post(path, bytes.NewReader(bodyBytes), &out); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action":     "reply",
			"comment_id": commentID,
			"reply_id":   out.ID,
			"url":        out.HTMLURL,
		})
	}
	fmt.Printf("replied: comment=%d reply=%d %s\n", commentID, out.ID, out.HTMLURL)
	return nil
}

func runResolve(args []string, undo bool) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	var threadID string
	var jsonOut bool
	fs.StringVar(&threadID, "thread", "", "review thread node ID (PRRT_...)")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if threadID == "" {
		return errors.New("--thread is required")
	}
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return err
	}
	query := resolveThreadMutation
	if undo {
		query = unresolveThreadMutation
	}
	vars := map[string]interface{}{"threadId": threadID}
	var out map[string]any
	if err := gql.Do(query, vars, &out); err != nil {
		return err
	}
	if jsonOut {
		action := "resolve"
		if undo {
			action = "unresolve"
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action":    action,
			"thread_id": threadID,
		})
	}
	if undo {
		fmt.Printf("unresolved: %s\n", threadID)
	} else {
		fmt.Printf("resolved: %s\n", threadID)
	}
	return nil
}

func resolveBodyText(body, bodyFile string) (string, error) {
	if body != "" && bodyFile != "" {
		return "", errors.New("use either --body or --body-file, not both")
	}
	if bodyFile != "" {
		b, err := os.ReadFile(bodyFile)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if body != "" {
		return body, nil
	}
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func resolveRepo(repoArg string) (string, string, error) {
	if repoArg != "" {
		parts := strings.Split(repoArg, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid --repo %q, expected owner/name", repoArg)
		}
		return parts[0], parts[1], nil
	}
	if owner, name, ok := inferRepoFromUpstream(); ok {
		return owner, name, nil
	}
	repo, err := repository.Current()
	if err != nil {
		return "", "", err
	}
	return repo.Owner, repo.Name, nil
}

func inferRepoFromUpstream() (string, string, bool) {
	upstream, err := runGit("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimSpace(upstream), "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	remoteName := parts[0]
	remoteURL, err := runGit("remote", "get-url", remoteName)
	if err != nil {
		return "", "", false
	}
	owner, name, ok := parseGitHubRepo(remoteURL)
	if !ok {
		return "", "", false
	}
	return owner, name, true
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseGitHubRepo(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ".git")
	if strings.HasPrefix(raw, "git@") {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return "", "", false
		}
		path := strings.TrimPrefix(parts[1], "/")
		p := strings.Split(path, "/")
		if len(p) < 2 {
			return "", "", false
		}
		return p[len(p)-2], p[len(p)-1], true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	path := strings.Trim(u.Path, "/")
	p := strings.Split(path, "/")
	if len(p) < 2 {
		return "", "", false
	}
	return p[len(p)-2], p[len(p)-1], true
}

func fetchReviewThreads(owner, repo string, pr int) ([]threadRecord, error) {
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, err
	}

	var records []threadRecord
	var after *string
	for {
		var resp gqlThreadsResponse
		vars := map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": pr,
			"after":  after,
		}
		if err := gql.Do(listThreadsQuery, vars, &resp); err != nil {
			return nil, err
		}
		for _, n := range resp.Repository.PullRequest.ReviewThreads.Nodes {
			records = append(records, mapThreadNode(n))
		}
		if !resp.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage ||
			resp.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor == nil {
			break
		}
		after = resp.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor
	}
	return records, nil
}

func mapThreadNode(n threadNode) threadRecord {
	root := pickRootComment(n.Comments.Nodes)
	line := 0
	if n.Line != nil {
		line = *n.Line
	} else if n.OriginalLine != nil {
		line = *n.OriginalLine
	}
	created := parseTS(root.CreatedAt)
	updated := created
	for _, c := range n.Comments.Nodes {
		t := parseTS(c.CreatedAt)
		if t.After(updated) {
			updated = t
		}
	}
	return threadRecord{
		ThreadID:    n.ID,
		CommentID:   root.DatabaseID,
		Resolved:    n.IsResolved,
		Outdated:    n.IsOutdated,
		Path:        n.Path,
		Line:        line,
		Author:      root.Author.Login,
		Summary:     summarize(root.Body),
		Severity:    severityFromBody(root.Body),
		CreatedAt:   created,
		UpdatedAt:   updated,
		URL:         root.URL,
		CommentBody: root.Body,
		AllComments: n.Comments.Nodes,
	}
}

func pickRootComment(comments []commentNode) commentNode {
	for _, c := range comments {
		if c.ReplyTo == nil {
			return c
		}
	}
	if len(comments) > 0 {
		return comments[0]
	}
	return commentNode{}
}

func parseTS(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func severityFromBody(body string) string {
	lower := strings.ToLower(body)
	switch {
	case strings.Contains(lower, "critical"):
		return "critical"
	case strings.Contains(lower, "major"):
		return "major"
	case strings.Contains(lower, "minor"):
		return "minor"
	default:
		return ""
	}
}

func summarize(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "* ")
		line = strings.Trim(line, "*`_")
		if len(line) > 120 {
			return line[:117] + "..."
		}
		return line
	}
	return ""
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func filterRecords(records []threadRecord, opts listOptions) []threadRecord {
	state := strings.ToLower(strings.TrimSpace(opts.State))
	out := records[:0]
	for _, r := range records {
		if state == "unresolved" && r.Resolved {
			continue
		}
		if state == "resolved" && !r.Resolved {
			continue
		}
		if opts.PathLike != "" && !containsFold(r.Path, opts.PathLike) {
			continue
		}
		if opts.Author != "" && !containsFold(r.Author, opts.Author) {
			continue
		}
		if opts.Contains != "" {
			matched := containsFold(r.CommentBody, opts.Contains) || containsFold(r.Summary, opts.Contains)
			if !matched {
				for _, c := range r.AllComments {
					if containsFold(c.Body, opts.Contains) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}
		if opts.Severity != "" && !strings.EqualFold(r.Severity, opts.Severity) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func sortRecords(records []threadRecord, sortBy string, desc bool) {
	key := strings.ToLower(strings.TrimSpace(sortBy))
	less := func(i, j int) bool {
		a, b := records[i], records[j]
		switch key {
		case "created":
			return a.CreatedAt.Before(b.CreatedAt)
		case "path":
			if a.Path == b.Path {
				return a.Line < b.Line
			}
			return a.Path < b.Path
		case "line":
			if a.Path == b.Path {
				return a.Line < b.Line
			}
			return a.Path < b.Path
		case "author":
			return strings.ToLower(a.Author) < strings.ToLower(b.Author)
		case "severity":
			return severityRank(a.Severity) < severityRank(b.Severity)
		case "updated":
			fallthrough
		default:
			return a.UpdatedAt.Before(b.UpdatedAt)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if desc {
			return !less(i, j)
		}
		return less(i, j)
	})
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 1
	case "major":
		return 2
	case "minor":
		return 3
	default:
		return 4
	}
}

func printTable(records []threadRecord) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "THREAD\tCOMMENT\tSTATE\tOUTDATED\tFILE\tAUTHOR\tUPDATED\tSEVERITY\tSUMMARY")
	for _, r := range records {
		state := "open"
		if r.Resolved {
			state = "resolved"
		}
		file := r.Path
		if r.Line > 0 {
			file = file + ":" + strconv.Itoa(r.Line)
		}
		updated := "-"
		if !r.UpdatedAt.IsZero() {
			updated = r.UpdatedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(
			w,
			"%s\t%d\t%s\t%t\t%s\t%s\t%s\t%s\t%s\n",
			r.ThreadID, r.CommentID, state, r.Outdated, file, r.Author, updated, r.Severity, r.Summary,
		)
	}
	_ = w.Flush()
}

func printStats(records []threadRecord) {
	bySeverity := map[string]int{}
	byAuthor := map[string]int{}
	byPath := map[string]int{}
	for _, r := range records {
		sev := r.Severity
		if sev == "" {
			sev = "unspecified"
		}
		bySeverity[sev]++
		byAuthor[r.Author]++
		byPath[r.Path]++
	}
	fmt.Printf("stats: total=%d\n", len(records))
	fmt.Printf("  by_severity: %s\n", compactMap(bySeverity))
	fmt.Printf("  by_author:   %s\n", compactMap(byAuthor))
	fmt.Printf("  by_path:     %s\n", compactMap(byPath))
}

func compactMap(m map[string]int) string {
	if len(m) == 0 {
		return "-"
	}
	type pair struct {
		k string
		v int
	}
	pairs := make([]pair, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, pair{k: k, v: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v == pairs[j].v {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v > pairs[j].v
	})
	var buf bytes.Buffer
	for i, p := range pairs {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(p.k)
		buf.WriteString("=")
		buf.WriteString(strconv.Itoa(p.v))
	}
	return buf.String()
}
