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

const usage = `loom - sift and action GitHub PR comments quickly

Usage:
  loom help [command...]
  loom list --pr <number> [flags]
  loom find --pr <number> --query <text> [flags]
  loom comment <subcommand> [flags]
  loom issue <subcommand> [flags]
  loom pr <subcommand> [flags]
  loom thread <subcommand> [flags]

Comment subcommands:
  loom comment top --pr <number> (--body <text> | --body-file <file>) [flags]
  loom comment inline --pr <number> --path <file> --line <n> --side <LEFT|RIGHT> (--body <text> | --body-file <file>) [flags]
  loom comment file --pr <number> --path <file> (--body <text> | --body-file <file>) [flags]
  loom comment edit --comment-id <id-or-url> (--body <text> | --body-file <file>) [flags]
  loom comment delete --comment-id <id-or-url> [flags]
  loom comment reply --pr <number> --comment-id <id-or-url> (--body <text> | --body-file <file>) [flags]

Issue subcommands:
  loom issue create --title <text> (--body <text> | --body-file <file>) [flags]
  loom issue close --issue <number> [flags]

PR subcommands:
  loom pr create --title <text> (--body <text> | --body-file <file>) [flags]
  loom pr edit --pr <number> [flags]
  loom pr merge --pr <number> [flags]

Thread subcommands:
  loom thread resolve --thread-id <node-id> [--repo <owner/name> --pr <number> --comment <id-or-url>]
  loom thread unresolve --thread-id <node-id> [--repo <owner/name> --pr <number> --comment <id-or-url>]

Common flags:
  --repo <owner/name>   Repository (default: current git remote)

Examples:
  loom list --repo <owner/repo> --pr <pr-number> --format table
  loom list --pr <pr-number> --state unresolved --severity major --sort created --desc
  loom list --pr <pr-number> --query "<text>" --path <path/to/file>
  loom comment top --pr <pr-number> --body "Top-level PR note"
  loom comment inline --pr <pr-number> --path <path/to/file> --line <line-number> --side RIGHT --body "Please rename this."
  loom comment file --pr <pr-number> --path <path/to/file> --body "This file needs an inline usage example."
  loom comment edit --repo <owner/repo> --comment-id <comment-id> --body "Updated wording"
  loom comment delete --repo <owner/repo> --comment-id <comment-id>
  loom comment reply --pr <pr-number> --comment-id <comment-id> --body "Addressed in <commit-url>"
  loom issue create --repo <owner/repo> --title "Tracking bug" --body "Details"
  loom issue close --repo <owner/repo> --issue <issue-number> --reason completed
  loom pr create --repo <owner/repo> --head <head-branch> --base <base-branch> --title "Ship it" --body "Summary"
  loom pr edit --repo <owner/repo> --pr <pr-number> --title "Updated title" --body "Updated summary"
  loom thread resolve --thread-id <thread-id>
  loom pr merge --repo <owner/repo> --pr <pr-number> --method squash

Use "loom help <command...>" or "loom <command> ... --help" for command details.
Legacy single-token aliases such as comment-top and pr-create remain supported.
`

const listHelp = `List pull request review threads.

USAGE
  loom list --pr <number> [flags]

ALIASES
  loom ls

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --pr <number>           Pull request number
  --state <value>         unresolved|resolved|all (default: unresolved)
  --path <substring>      Filter by file path substring
  --author <login>        Filter by root comment author
  --query <text>          Filter by body substring
  --contains <text>       Deprecated alias for --query
  --severity <level>      critical|major|minor
  --sort <field>          updated|created|path|line|author|severity (default: updated)
  --desc                  Sort descending (default: true)
  --limit <n>             Max rows to print (default: 200)
  --format <format>       auto|table|json|jsonl (default: auto)
  --json                  Deprecated alias for --format json
  --stats                 Print grouped summary to stderr

OUTPUT
  table                   Human-friendly thread summary
  json                    JSON array of thread records
  jsonl                   One JSON record per line

JSON FIELDS
  thread_id, comment_id, resolved, outdated, path, line, author,
  summary, severity, created_at, updated_at, url, comment_body

EXAMPLES
  loom list --pr 24
  loom list --repo oinoom/loom --pr 24 --state all --format json
  loom list --pr 24 --path main.go --author octocat --severity major
  loom list --pr 24 --query "stale rows" --sort updated --desc
`

const findHelp = `Find review threads by text query.

USAGE
  loom find --pr <number> --query <text> [flags]

DESCRIPTION
  find is a thin wrapper around list that requires --query.
  Prefer "loom list --query ..." if you want one command shape for humans and agents.

FLAGS
  Uses the same flags as "loom list", with --query required.

OUTPUT
  Uses the same output formats and JSON fields as "loom list".

EXAMPLES
  loom find --pr <pr-number> --query "rename this"
  loom find --repo <owner/repo> --pr <pr-number> --query "stale rows" --path <path/to/file> --format json
`

const commentHelp = `Manage pull request comments.

USAGE
  loom comment <subcommand> [flags]

SUBCOMMANDS
  top                   Add a top-level PR comment
  inline                Add an inline review comment
  file                  Add a file-level review comment
  edit                  Edit an existing PR comment
  delete                Delete an existing PR comment
  reply                 Reply to a review comment

ALIASES
  Legacy single-token aliases remain supported:
  comment-top, comment-inline, comment-file, edit, delete, reply

EXAMPLES
  loom help comment inline
  loom comment top --pr <pr-number> --body "Top-level PR note"
  loom comment inline --pr <pr-number> --path <path/to/file> --line <line-number> --side RIGHT --body "Please rename this."
  loom comment file --pr <pr-number> --path <path/to/file> --body "This file needs an example."
  loom comment edit --comment-id <comment-id> --body "Updated wording"
  loom comment reply --pr <pr-number> --comment-id <comment-id> --body "Addressed in <commit-url>"
`

const commentTopHelp = `Add a top-level pull request comment.

USAGE
  loom comment top --pr <number> (--body <text> | --body-file <file>) [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --pr <number>           Pull request number
  --body <text>           Comment text
  --body-file <file>      Read comment text from file
  --json                  Output JSON

NOTES
  If --body and --body-file are omitted, loom reads from stdin.

EXAMPLES
  loom comment top --pr <pr-number> --body "Overall review note"
  loom comment top --repo <owner/repo> --pr <pr-number> --body-file /tmp/comment.txt --json
`

const commentInlineHelp = `Add an inline pull request review comment.

USAGE
  loom comment inline --pr <number> --path <file> --line <n> --side <LEFT|RIGHT> (--body <text> | --body-file <file>) [flags]
  loom comment inline --pr <number> --path <file> --start-line <n> --start-side <LEFT|RIGHT> --line <n> --side <LEFT|RIGHT> (--body <text> | --body-file <file>) [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --pr <number>           Pull request number
  --body <text>           Comment text
  --body-file <file>      Read comment text from file
  --path <file>           File path in the PR diff
  --commit <sha>          PR head commit SHA (auto-detected if omitted)
  --line <n>              Diff line to comment on
  --side <LEFT|RIGHT>     Diff side for --line
  --start-line <n>        Start line for multi-line comments
  --start-side <side>     Diff side for --start-line
  --json                  Output JSON

NOTES
  If --body and --body-file are omitted, loom reads from stdin.
  Use "loom comment file" for whole-file review comments.

EXAMPLES
  loom comment inline --pr <pr-number> --path <path/to/file> --line <line-number> --side RIGHT --body "Please rename this."
  loom comment inline --pr <pr-number> --path <path/to/file> --start-line <start-line> --start-side RIGHT --line <end-line> --side RIGHT --body "Needs more detail."
`

const commentFileHelp = `Add a file-level pull request review comment.

USAGE
  loom comment file --pr <number> --path <file> (--body <text> | --body-file <file>) [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --pr <number>           Pull request number
  --body <text>           Comment text
  --body-file <file>      Read comment text from file
  --path <file>           File path in the PR diff
  --commit <sha>          PR head commit SHA (auto-detected if omitted)
  --json                  Output JSON

NOTES
  "loom comment file" is equivalent to "loom comment inline --subject file".

EXAMPLES
  loom comment file --pr <pr-number> --path <path/to/file> --body "This file needs an inline usage example."
`

const editHelp = `Edit an existing PR comment.

USAGE
  loom comment edit --comment-id <id-or-url> (--body <text> | --body-file <file>) [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --comment <id-or-url>   Comment database ID or URL
  --comment-id <ref>      Preferred alias for --comment
  --url <url>             Comment URL
  --body <text>           Updated comment text
  --body-file <file>      Read updated comment text from file
  --type <value>          auto|review|top-level (default: auto)
  --json                  Output JSON

NOTES
  If --type is omitted, loom tries review comments first and then top-level PR comments.

EXAMPLES
  loom comment edit --repo <owner/repo> --comment-id <comment-id> --body "Updated wording"
  loom comment edit --comment-id https://github.com/<owner>/<repo>/pull/<pr-number>#discussion_r<comment-id> --body-file /tmp/comment.txt --json
`

const deleteHelp = `Delete an existing PR comment.

USAGE
  loom comment delete --comment-id <id-or-url> [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --comment <id-or-url>   Comment database ID or URL
  --comment-id <ref>      Preferred alias for --comment
  --url <url>             Comment URL
  --type <value>          auto|review|top-level (default: auto)
  --json                  Output JSON

NOTES
  If --type is omitted, loom tries review comments first and then top-level PR comments.

EXAMPLES
  loom comment delete --repo <owner/repo> --comment-id <comment-id>
  loom comment delete --comment-id https://github.com/<owner>/<repo>/pull/<pr-number>#issuecomment-<comment-id> --json
`

const replyHelp = `Reply to a pull request review comment.

USAGE
  loom comment reply --pr <number> --comment-id <id-or-url> (--body <text> | --body-file <file>) [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --pr <number>           Pull request number
  --comment <id-or-url>   Review comment database ID or URL
  --comment-id <ref>      Preferred alias for --comment
  --url <url>             Comment URL
  --body <text>           Reply text
  --body-file <file>      Read reply text from file
  --json                  Output JSON

NOTES
  If --body and --body-file are omitted, loom reads from stdin.

EXAMPLES
  loom comment reply --pr <pr-number> --comment-id <comment-id> --body "Addressed in <commit-url>"
  loom comment reply --repo <owner/repo> --pr <pr-number> --comment-id https://github.com/<owner>/<repo>/pull/<pr-number>#discussion_r<comment-id> --body-file /tmp/reply.txt --json
`

const resolveHelp = `Resolve a pull request review thread.

USAGE
  loom thread resolve --thread-id <PRRT_...> [flags]
  loom thread resolve --repo <owner/name> --pr <number> --comment-id <id-or-url> [flags]

FLAGS
  --thread <PRRT_...>     Review thread node ID
  --thread-id <id>        Preferred alias for --thread
  --repo <owner/name>     Repository
  --pr <number>           Pull request number when resolving by comment
  --comment <id-or-url>   Review comment database ID or URL
  --comment-id <ref>      Preferred alias for --comment
  --url <url>             Review comment URL
  --json                  Output JSON

DESCRIPTION
  Resolve directly by thread ID, or provide repo/pr/comment so loom can look up
  the owning thread before resolving it.

OUTPUT
  JSON mode returns action, thread_id, and comment_id when resolving by comment.

EXAMPLES
  loom thread resolve --thread-id <thread-id>
  loom thread resolve --repo <owner/repo> --pr <pr-number> --comment-id <comment-id> --json
`

const unresolveHelp = `Unresolve a pull request review thread.

USAGE
  loom thread unresolve --thread-id <PRRT_...> [flags]
  loom thread unresolve --repo <owner/name> --pr <number> --comment-id <id-or-url> [flags]

FLAGS
  Uses the same flags as "loom thread resolve".

OUTPUT
  JSON mode returns action, thread_id, and comment_id when unresolving by comment.

EXAMPLES
  loom thread unresolve --thread-id <thread-id>
  loom thread unresolve --repo <owner/repo> --pr <pr-number> --comment-id https://github.com/<owner>/<repo>/pull/<pr-number>#discussion_r<comment-id> --json
`

const mergeHelp = `Merge a pull request.

USAGE
  loom pr merge --pr <number> [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --pr <number>           Pull request number
  --method <value>        merge|squash|rebase (default: squash)
  --title <text>          Optional merge commit title
  --message <text>        Optional merge commit message
  --json                  Output JSON

OUTPUT
  JSON mode returns action, pr, method, merged, sha, and message.

EXAMPLES
  loom pr merge --pr <pr-number> --method squash
  loom pr merge --repo <owner/repo> --pr <pr-number> --method rebase --json
`

const issueHelp = `Manage issues.

USAGE
  loom issue <subcommand> [flags]

SUBCOMMANDS
  create                Create an issue
  close                 Close an issue

ALIASES
  Legacy single-token aliases remain supported:
  issue-close

EXAMPLES
  loom help issue create
  loom issue create --title "Tracking bug" --body "Details"
  loom issue close --issue <issue-number> --reason completed
`

const issueCreateHelp = `Create an issue.

USAGE
  loom issue create --title <text> (--body <text> | --body-file <file>) [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --title <text>          Issue title
  --body <text>           Issue body
  --body-file <file>      Read issue body from file
  --labels <csv>          Comma-separated labels
  --json                  Output JSON

NOTES
  If --body and --body-file are omitted, loom reads from stdin.

OUTPUT
  JSON mode returns action, issue, url, state, and title.

EXAMPLES
  loom issue create --title "Tracking bug" --body "Details"
  loom issue create --repo <owner/repo> --title "Backlog task" --body-file /tmp/issue.md --labels bug,docs --json
`

const issueCloseHelp = `Close an issue.

USAGE
  loom issue close --issue <number> [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --issue <number>        Issue number
  --reason <value>        completed|not_planned
  --json                  Output JSON

OUTPUT
  JSON mode returns action, issue, url, state, title, and reason when present.

EXAMPLES
  loom issue close --issue <issue-number>
  loom issue close --repo <owner/repo> --issue <issue-number> --reason completed --json
`

const prHelp = `Manage pull requests.

USAGE
  loom pr <subcommand> [flags]

SUBCOMMANDS
  create                Create a pull request
  edit                  Edit a pull request
  merge                 Merge a pull request

ALIASES
  Legacy single-token aliases remain supported:
  pr-create, pr-edit, merge

EXAMPLES
  loom help pr create
  loom pr create --head <head-branch> --base <base-branch> --title "Ship it" --body "Summary"
  loom pr merge --pr <pr-number> --method squash
`

const prCreateHelp = `Create a pull request.

USAGE
  loom pr create --title <text> (--body <text> | --body-file <file>) [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --title <text>          Pull request title
  --body <text>           Pull request body
  --body-file <file>      Read pull request body from file
  --head <branch>         Head branch (default: current branch)
  --base <branch>         Base branch (default: main)
  --draft                 Create as draft
  --json                  Output JSON

NOTES
  If --body and --body-file are omitted, loom reads from stdin.

OUTPUT
  JSON mode returns action, pr, url, state, title, and draft.

EXAMPLES
  loom pr create --title "Ship it" --body "Summary"
  loom pr create --repo <owner/repo> --head <head-branch> --base <base-branch> --title "Ship it" --body-file /tmp/pr.md --draft --json
`

const prEditHelp = `Edit a pull request.

USAGE
  loom pr edit --pr <number> [flags]

FLAGS
  --repo <owner/name>     Repository (default: current git remote)
  --pr <number>           Pull request number
  --title <text>          Updated pull request title
  --body <text>           Updated pull request body
  --body-file <file>      Read updated pull request body from file
  --base <branch>         Updated base branch
  --json                  Output JSON

NOTES
  Provide at least one of --title, --body/--body-file, or --base.

OUTPUT
  JSON mode returns action, pr, url, state, and title.

EXAMPLES
  loom pr edit --pr <pr-number> --title "Updated title"
  loom pr edit --repo <owner/repo> --pr <pr-number> --body-file /tmp/pr.md --base <base-branch> --json
`

const threadHelp = `Manage pull request review threads.

USAGE
  loom thread <subcommand> [flags]

SUBCOMMANDS
  resolve               Resolve a review thread
  unresolve             Re-open a review thread

ALIASES
  Legacy single-token aliases remain supported:
  resolve, unresolve

EXAMPLES
  loom help thread resolve
  loom thread resolve --thread-id <thread-id>
  loom thread unresolve --repo <owner/repo> --pr <pr-number> --comment-id <comment-id>
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
	Format   string
	Stats    bool
}

type pullRequestResponse struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type inlineCommentRequest struct {
	Body        string `json:"body"`
	CommitID    string `json:"commit_id"`
	Path        string `json:"path"`
	SubjectType string `json:"subject_type,omitempty"`
	Line        int    `json:"line,omitempty"`
	Side        string `json:"side,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
	StartSide   string `json:"start_side,omitempty"`
}

type reviewCommentResponse struct {
	ID          int64  `json:"id"`
	NodeID      string `json:"node_id"`
	HTMLURL     string `json:"html_url"`
	Path        string `json:"path"`
	Line        *int   `json:"line"`
	StartLine   *int   `json:"start_line"`
	Side        string `json:"side"`
	StartSide   string `json:"start_side"`
	CommitID    string `json:"commit_id"`
	SubjectType string `json:"subject_type"`
}

type commentResponse struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"html_url"`
}

type issueResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Title   string `json:"title"`
}

type pullRequestCreateResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Title   string `json:"title"`
}

type mergeResponse struct {
	SHA     string `json:"sha"`
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

type restAPIError struct {
	StatusCode int
	Message    string
}

func (e *restAPIError) Error() string {
	return e.Message
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func isHelpToken(arg string) bool {
	switch strings.TrimSpace(arg) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func wantsCommandHelp(args []string) bool {
	return len(args) == 1 && isHelpToken(args[0])
}

func canonicalCommandPath(cmd string) string {
	path := strings.Join(strings.Fields(strings.TrimSpace(cmd)), " ")
	switch path {
	case "ls":
		return "list"
	case "comment-top":
		return "comment top"
	case "comment-inline":
		return "comment inline"
	case "comment-file":
		return "comment file"
	case "edit":
		return "comment edit"
	case "delete":
		return "comment delete"
	case "reply":
		return "comment reply"
	case "issue-close":
		return "issue close"
	case "pr-create":
		return "pr create"
	case "pr-edit":
		return "pr edit"
	case "merge":
		return "pr merge"
	case "resolve":
		return "thread resolve"
	case "unresolve":
		return "thread unresolve"
	default:
		return path
	}
}

func commandHelpText(cmd string) (string, bool) {
	switch canonicalCommandPath(cmd) {
	case "list":
		return listHelp, true
	case "find":
		return findHelp, true
	case "comment":
		return commentHelp, true
	case "comment top":
		return commentTopHelp, true
	case "comment inline":
		return commentInlineHelp, true
	case "comment file":
		return commentFileHelp, true
	case "comment edit":
		return editHelp, true
	case "comment delete":
		return deleteHelp, true
	case "comment reply":
		return replyHelp, true
	case "issue":
		return issueHelp, true
	case "issue create":
		return issueCreateHelp, true
	case "issue close":
		return issueCloseHelp, true
	case "pr":
		return prHelp, true
	case "pr create":
		return prCreateHelp, true
	case "pr edit":
		return prEditHelp, true
	case "pr merge":
		return mergeHelp, true
	case "thread":
		return threadHelp, true
	case "thread resolve":
		return resolveHelp, true
	case "thread unresolve":
		return unresolveHelp, true
	default:
		return "", false
	}
}

func printCommandHelp(cmd string) bool {
	if text, ok := commandHelpText(cmd); ok {
		fmt.Print(text)
		return true
	}
	return false
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	if isHelpToken(cmd) {
		if len(args) > 0 && printCommandHelp(strings.Join(args, " ")) {
			return
		}
		fmt.Print(usage)
		return
	}

	var err error
	switch cmd {
	case "list", "ls":
		err = runList(args, "")
	case "find":
		err = runFind(args)
	case "comment":
		err = runCommentGroup(args)
	case "comment-top":
		err = runComment(args, "comment top", "")
	case "comment-inline":
		err = runComment(args, "comment inline", "")
	case "comment-file":
		err = runComment(args, "comment file", "file")
	case "edit":
		err = runEdit(args)
	case "delete":
		err = runDelete(args)
	case "reply":
		err = runReply(args)
	case "issue":
		err = runIssueGroup(args)
	case "issue-close":
		err = runIssueClose(args)
	case "pr":
		err = runPRGroup(args)
	case "pr-create":
		err = runPRCreate(args)
	case "pr-edit":
		err = runPREdit(args)
	case "thread":
		err = runThreadGroup(args)
	case "resolve":
		err = runResolve(args, "thread resolve", false)
	case "unresolve":
		err = runResolve(args, "thread unresolve", true)
	case "merge":
		err = runMerge(args)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runCommentGroup(args []string) error {
	if len(args) == 0 || wantsCommandHelp(args) {
		fmt.Print(commentHelp)
		return nil
	}
	if isHelpToken(args[0]) {
		if len(args) > 1 && printCommandHelp("comment "+strings.Join(args[1:], " ")) {
			return nil
		}
		fmt.Print(commentHelp)
		return nil
	}

	switch args[0] {
	case "top":
		return runComment(args[1:], "comment top", "")
	case "inline":
		return runComment(args[1:], "comment inline", "")
	case "file":
		return runComment(args[1:], "comment file", "file")
	case "edit":
		return runEdit(args[1:])
	case "delete":
		return runDelete(args[1:])
	case "reply":
		return runReply(args[1:])
	default:
		return fmt.Errorf("unknown comment subcommand %q", args[0])
	}
}

func runIssueGroup(args []string) error {
	if len(args) == 0 || wantsCommandHelp(args) {
		fmt.Print(issueHelp)
		return nil
	}
	if isHelpToken(args[0]) {
		if len(args) > 1 && printCommandHelp("issue "+strings.Join(args[1:], " ")) {
			return nil
		}
		fmt.Print(issueHelp)
		return nil
	}

	switch args[0] {
	case "create":
		return runIssue(args[1:])
	case "close":
		return runIssueClose(args[1:])
	default:
		return fmt.Errorf("unknown issue subcommand %q", args[0])
	}
}

func runPRGroup(args []string) error {
	if len(args) == 0 || wantsCommandHelp(args) {
		fmt.Print(prHelp)
		return nil
	}
	if isHelpToken(args[0]) {
		if len(args) > 1 && printCommandHelp("pr "+strings.Join(args[1:], " ")) {
			return nil
		}
		fmt.Print(prHelp)
		return nil
	}

	switch args[0] {
	case "create":
		return runPRCreate(args[1:])
	case "edit":
		return runPREdit(args[1:])
	case "merge":
		return runMerge(args[1:])
	default:
		return fmt.Errorf("unknown pr subcommand %q", args[0])
	}
}

func runThreadGroup(args []string) error {
	if len(args) == 0 || wantsCommandHelp(args) {
		fmt.Print(threadHelp)
		return nil
	}
	if isHelpToken(args[0]) {
		if len(args) > 1 && printCommandHelp("thread "+strings.Join(args[1:], " ")) {
			return nil
		}
		fmt.Print(threadHelp)
		return nil
	}

	switch args[0] {
	case "resolve":
		return runResolve(args[1:], "thread resolve", false)
	case "unresolve":
		return runResolve(args[1:], "thread unresolve", true)
	default:
		return fmt.Errorf("unknown thread subcommand %q", args[0])
	}
}

func runFind(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(findHelp)
		return nil
	}
	fs := newFlagSet("find")
	var opts listOptions
	fs.StringVar(&opts.Repo, "repo", "", "owner/name repository")
	fs.IntVar(&opts.PR, "pr", 0, "pull request number")
	fs.StringVar(&opts.Contains, "query", "", "text to search in review comments")
	fs.StringVar(&opts.Contains, "contains", "", "deprecated alias for --query")
	fs.StringVar(&opts.PathLike, "path", "", "filter by file path substring")
	fs.StringVar(&opts.Author, "author", "", "filter by root comment author")
	fs.StringVar(&opts.State, "state", "unresolved", "unresolved|resolved|all")
	fs.StringVar(&opts.Severity, "severity", "", "critical|major|minor")
	fs.StringVar(&opts.SortBy, "sort", "updated", "updated|created|path|line|author|severity")
	fs.BoolVar(&opts.Desc, "desc", true, "descending sort")
	fs.IntVar(&opts.Limit, "limit", 200, "max rows to print")
	fs.StringVar(&opts.Format, "format", "auto", "output format: auto|table|json|jsonl")
	fs.BoolVar(&opts.JSON, "json", false, "deprecated alias for --format json")
	fs.BoolVar(&opts.Stats, "stats", false, "print grouped summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(opts.Contains) == "" {
		return errors.New("--query is required")
	}
	return executeList(opts)
}

func runList(args []string, presetContains string) error {
	if wantsCommandHelp(args) {
		fmt.Print(listHelp)
		return nil
	}
	fs := newFlagSet("list")
	var opts listOptions
	fs.StringVar(&opts.Repo, "repo", "", "owner/name repository")
	fs.IntVar(&opts.PR, "pr", 0, "pull request number")
	fs.StringVar(&opts.State, "state", "unresolved", "unresolved|resolved|all")
	fs.StringVar(&opts.PathLike, "path", "", "filter by file path substring")
	fs.StringVar(&opts.Author, "author", "", "filter by root comment author")
	fs.StringVar(&opts.Contains, "query", presetContains, "filter by body substring")
	fs.StringVar(&opts.Contains, "contains", presetContains, "deprecated alias for --query")
	fs.StringVar(&opts.Severity, "severity", "", "critical|major|minor")
	fs.StringVar(&opts.SortBy, "sort", "updated", "updated|created|path|line|author|severity")
	fs.BoolVar(&opts.Desc, "desc", true, "descending sort")
	fs.IntVar(&opts.Limit, "limit", 200, "max rows to print")
	fs.StringVar(&opts.Format, "format", "auto", "output format: auto|table|json|jsonl")
	fs.BoolVar(&opts.JSON, "json", false, "deprecated alias for --format json")
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

	format, err := resolveListFormat(opts.Format, opts.JSON)
	if err != nil {
		return err
	}
	if opts.Stats {
		printStats(os.Stderr, records)
	}
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	case "jsonl":
		return printJSONL(records)
	default:
		printTable(records)
		return nil
	}
}

func runComment(args []string, helpName string, defaultSubject string) error {
	if wantsCommandHelp(args) {
		printCommandHelp(helpName)
		return nil
	}
	fs := newFlagSet(helpName)
	var repoArg string
	var pr int
	var body string
	var bodyFile string
	var pathArg string
	var commitID string
	var subjectType string
	var line int
	var side string
	var startLine int
	var startSide string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.IntVar(&pr, "pr", 0, "pull request number")
	fs.StringVar(&body, "body", "", "comment text")
	fs.StringVar(&bodyFile, "body-file", "", "read comment text from file")
	fs.StringVar(&pathArg, "path", "", "file path for inline review comment")
	fs.StringVar(&commitID, "commit", "", "pull request head commit SHA (auto-detected if omitted for inline comments)")
	fs.StringVar(&subjectType, "subject", "", `inline comment subject type (supported: "file")`)
	fs.IntVar(&line, "line", 0, "line in the pull request diff for inline comments")
	fs.StringVar(&side, "side", "", "diff side for inline comments: LEFT or RIGHT")
	fs.IntVar(&startLine, "start-line", 0, "start line for multi-line inline comments")
	fs.StringVar(&startSide, "start-side", "", "start diff side for multi-line inline comments: LEFT or RIGHT")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(defaultSubject) != "" && strings.TrimSpace(subjectType) == "" {
		subjectType = defaultSubject
	}
	if pr <= 0 {
		return errors.New("--pr is required")
	}
	text, err := resolveBodyText(body, bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("comment body is empty; pass --body, --body-file, or pipe stdin")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	if strings.TrimSpace(pathArg) == "" {
		if commitID != "" || subjectType != "" || line > 0 || side != "" || startLine > 0 || startSide != "" {
			return errors.New("inline comment flags require --path; for top-level PR comments use only --body/--body-file")
		}
		return postTopLevelComment(client, owner, repo, pr, text, jsonOut)
	}

	req, err := buildInlineCommentRequest(client, owner, repo, pr, text, pathArg, commitID, subjectType, line, side, startLine, startSide)
	if err != nil {
		return err
	}
	return postInlineComment(client, owner, repo, pr, req, jsonOut)
}

func runEdit(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(editHelp)
		return nil
	}
	fs := newFlagSet("edit")
	var repoArg string
	var commentRef string
	var body string
	var bodyFile string
	var commentType string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.StringVar(&commentRef, "comment", "", "comment database ID or URL")
	fs.StringVar(&commentRef, "comment-id", "", "comment database ID or URL")
	fs.StringVar(&commentRef, "url", "", "comment URL")
	fs.StringVar(&body, "body", "", "updated comment text")
	fs.StringVar(&bodyFile, "body-file", "", "read updated comment text from file")
	fs.StringVar(&commentType, "type", "auto", `comment type: "auto", "review", or "top-level"`)
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	commentID, err := parseCommentReference(commentRef)
	if err != nil {
		return errors.New("--comment is required")
	}
	text, err := resolveBodyText(body, bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("comment body is empty; pass --body, --body-file, or pipe stdin")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	resolvedType, out, err := editComment(client, owner, repo, commentID, commentType, text)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action":     "edit",
			"type":       resolvedType,
			"comment_id": out.ID,
			"url":        out.HTMLURL,
		})
	}
	fmt.Printf("edited: type=%s comment=%d %s\n", resolvedType, out.ID, out.HTMLURL)
	return nil
}

func runDelete(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(deleteHelp)
		return nil
	}
	fs := newFlagSet("delete")
	var repoArg string
	var commentRef string
	var commentType string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.StringVar(&commentRef, "comment", "", "comment database ID or URL")
	fs.StringVar(&commentRef, "comment-id", "", "comment database ID or URL")
	fs.StringVar(&commentRef, "url", "", "comment URL")
	fs.StringVar(&commentType, "type", "auto", `comment type: "auto", "review", or "top-level"`)
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	commentID, err := parseCommentReference(commentRef)
	if err != nil {
		return errors.New("--comment is required")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	resolvedType, err := deleteComment(client, owner, repo, commentID, commentType)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action":     "delete",
			"type":       resolvedType,
			"comment_id": commentID,
		})
	}
	fmt.Printf("deleted: type=%s comment=%d\n", resolvedType, commentID)
	return nil
}

func runMerge(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(mergeHelp)
		return nil
	}
	fs := newFlagSet("merge")
	var repoArg string
	var pr int
	var method string
	var title string
	var message string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.IntVar(&pr, "pr", 0, "pull request number")
	fs.StringVar(&method, "method", "squash", "merge method: merge|squash|rebase")
	fs.StringVar(&title, "title", "", "optional merge commit title")
	fs.StringVar(&message, "message", "", "optional merge commit message")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if pr <= 0 {
		return errors.New("--pr is required")
	}
	mergeMethod, err := normalizeMergeMethod(method)
	if err != nil {
		return err
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	req := map[string]string{"merge_method": mergeMethod}
	if strings.TrimSpace(title) != "" {
		req["commit_title"] = strings.TrimSpace(title)
	}
	if strings.TrimSpace(message) != "" {
		req["commit_message"] = strings.TrimSpace(message)
	}

	var out mergeResponse
	if err := doRESTJSON(client, "PUT", fmt.Sprintf("repos/%s/%s/pulls/%d/merge", owner, repo, pr), req, &out); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action":  "merge",
			"pr":      pr,
			"method":  mergeMethod,
			"merged":  out.Merged,
			"sha":     out.SHA,
			"message": out.Message,
		})
	}
	fmt.Printf("merged: pr=%d method=%s sha=%s %s\n", pr, mergeMethod, out.SHA, out.Message)
	return nil
}

func runReply(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(replyHelp)
		return nil
	}
	fs := newFlagSet("reply")
	var repoArg string
	var pr int
	var commentRef string
	var body string
	var bodyFile string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.IntVar(&pr, "pr", 0, "pull request number")
	fs.StringVar(&commentRef, "comment", "", "pull request review comment database ID or URL")
	fs.StringVar(&commentRef, "comment-id", "", "pull request review comment database ID or URL")
	fs.StringVar(&commentRef, "url", "", "comment URL")
	fs.StringVar(&body, "body", "", "reply text")
	fs.StringVar(&bodyFile, "body-file", "", "read reply text from file")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if pr <= 0 {
		return errors.New("--pr is required")
	}
	commentID, err := parseCommentReference(commentRef)
	if err != nil {
		return errors.New("--comment is required")
	}
	text, err := resolveBodyText(body, bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("reply body is empty; pass --body, --body-file, or pipe stdin")
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

func runIssue(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(issueCreateHelp)
		return nil
	}
	fs := newFlagSet("issue")
	var repoArg string
	var title string
	var body string
	var bodyFile string
	var labelsRaw string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.StringVar(&title, "title", "", "issue title")
	fs.StringVar(&body, "body", "", "issue body")
	fs.StringVar(&bodyFile, "body-file", "", "read issue body from file")
	fs.StringVar(&labelsRaw, "labels", "", "comma-separated labels")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("--title is required")
	}
	text, err := resolveBodyText(body, bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("issue body is empty; pass --body, --body-file, or pipe stdin")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	req := map[string]interface{}{
		"title": title,
		"body":  text,
	}
	labels := splitCSV(labelsRaw)
	if len(labels) > 0 {
		req["labels"] = labels
	}

	var out issueResponse
	if err := doRESTJSON(client, "POST", fmt.Sprintf("repos/%s/%s/issues", owner, repo), req, &out); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action": "issue",
			"issue":  out.Number,
			"url":    out.HTMLURL,
			"state":  out.State,
			"title":  out.Title,
		})
	}
	fmt.Printf("issue-created: issue=%d %s\n", out.Number, out.HTMLURL)
	return nil
}

func runIssueClose(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(issueCloseHelp)
		return nil
	}
	fs := newFlagSet("issue-close")
	var repoArg string
	var issue int
	var reason string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.IntVar(&issue, "issue", 0, "issue number")
	fs.StringVar(&reason, "reason", "", "optional state reason: completed|not_planned")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if issue <= 0 {
		return errors.New("--issue is required")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	req := map[string]interface{}{"state": "closed"}
	normalizedReason, err := normalizeIssueCloseReason(reason)
	if err != nil {
		return err
	}
	if normalizedReason != "" {
		req["state_reason"] = normalizedReason
	}

	var out issueResponse
	if err := doRESTJSON(client, "PATCH", fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, issue), req, &out); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		payload := map[string]interface{}{
			"action": "issue-close",
			"issue":  out.Number,
			"url":    out.HTMLURL,
			"state":  out.State,
			"title":  out.Title,
		}
		if normalizedReason != "" {
			payload["reason"] = normalizedReason
		}
		return enc.Encode(payload)
	}
	if normalizedReason != "" {
		fmt.Printf("issue-closed: issue=%d reason=%s %s\n", out.Number, normalizedReason, out.HTMLURL)
		return nil
	}
	fmt.Printf("issue-closed: issue=%d %s\n", out.Number, out.HTMLURL)
	return nil
}

func runPRCreate(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(prCreateHelp)
		return nil
	}
	fs := newFlagSet("pr-create")
	var repoArg string
	var title string
	var body string
	var bodyFile string
	var head string
	var base string
	var draft bool
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.StringVar(&title, "title", "", "pull request title")
	fs.StringVar(&body, "body", "", "pull request body")
	fs.StringVar(&bodyFile, "body-file", "", "read pull request body from file")
	fs.StringVar(&head, "head", "", "head branch (default: current branch)")
	fs.StringVar(&base, "base", "main", "base branch")
	fs.BoolVar(&draft, "draft", false, "create as draft")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("--title is required")
	}
	text, err := resolveBodyText(body, bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("pull request body is empty; pass --body, --body-file, or pipe stdin")
	}
	head = strings.TrimSpace(head)
	if head == "" {
		head, err = currentGitBranch()
		if err != nil {
			return err
		}
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return errors.New("--base is required")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	req := map[string]interface{}{
		"title": title,
		"body":  text,
		"head":  head,
		"base":  base,
		"draft": draft,
	}

	var out pullRequestCreateResponse
	if err := doRESTJSON(client, "POST", fmt.Sprintf("repos/%s/%s/pulls", owner, repo), req, &out); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action": "pr-create",
			"pr":     out.Number,
			"url":    out.HTMLURL,
			"state":  out.State,
			"title":  out.Title,
			"draft":  draft,
		})
	}
	fmt.Printf("pr-created: pr=%d %s\n", out.Number, out.HTMLURL)
	return nil
}

func runPREdit(args []string) error {
	if wantsCommandHelp(args) {
		fmt.Print(prEditHelp)
		return nil
	}
	fs := newFlagSet("pr-edit")
	var repoArg string
	var pr int
	var title string
	var body string
	var bodyFile string
	var base string
	var jsonOut bool
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.IntVar(&pr, "pr", 0, "pull request number")
	fs.StringVar(&title, "title", "", "updated pull request title")
	fs.StringVar(&body, "body", "", "updated pull request body")
	fs.StringVar(&bodyFile, "body-file", "", "read updated pull request body from file")
	fs.StringVar(&base, "base", "", "updated base branch")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if pr <= 0 {
		return errors.New("--pr is required")
	}

	req := map[string]interface{}{}
	if strings.TrimSpace(title) != "" {
		req["title"] = strings.TrimSpace(title)
	}
	if body != "" || bodyFile != "" {
		text, err := resolveBodyText(body, bodyFile)
		if err != nil {
			return err
		}
		if strings.TrimSpace(text) == "" {
			return errors.New("pull request body is empty; pass --body, --body-file, or pipe stdin")
		}
		req["body"] = text
	}
	if strings.TrimSpace(base) != "" {
		req["base"] = strings.TrimSpace(base)
	}
	if len(req) == 0 {
		return errors.New("provide at least one of --title, --body/--body-file, or --base")
	}

	owner, repo, err := resolveRepo(repoArg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	var out pullRequestCreateResponse
	if err := doRESTJSON(client, "PATCH", fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, pr), req, &out); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"action": "pr-edit",
			"pr":     out.Number,
			"url":    out.HTMLURL,
			"state":  out.State,
			"title":  out.Title,
		})
	}
	fmt.Printf("pr-edited: pr=%d %s\n", pr, out.HTMLURL)
	return nil
}

func runResolve(args []string, helpName string, undo bool) error {
	if wantsCommandHelp(args) {
		printCommandHelp(helpName)
		return nil
	}
	fs := newFlagSet(helpName)
	var threadID string
	var repoArg string
	var pr int
	var commentRef string
	var jsonOut bool
	var err error
	fs.StringVar(&threadID, "thread", "", "review thread node ID (PRRT_...)")
	fs.StringVar(&threadID, "thread-id", "", "review thread node ID (PRRT_...)")
	fs.StringVar(&repoArg, "repo", "", "owner/name repository")
	fs.IntVar(&pr, "pr", 0, "pull request number (required when resolving by comment)")
	fs.StringVar(&commentRef, "comment", "", "review comment database ID or URL")
	fs.StringVar(&commentRef, "comment-id", "", "review comment database ID or URL")
	fs.StringVar(&commentRef, "url", "", "review comment URL")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var resolvedCommentID int64
	if strings.TrimSpace(threadID) == "" {
		if pr <= 0 {
			return errors.New("--thread-id is required unless --repo, --pr, and --comment are provided")
		}
		resolvedCommentID, err = parseCommentReference(commentRef)
		if err != nil {
			return errors.New("--thread-id is required unless --repo, --pr, and --comment are provided")
		}
		owner, repo, err := resolveRepo(repoArg)
		if err != nil {
			return err
		}
		records, err := fetchReviewThreads(owner, repo, pr)
		if err != nil {
			return err
		}
		threadID, err = findThreadIDByComment(records, resolvedCommentID)
		if err != nil {
			return err
		}
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
			"action":     action,
			"thread_id":  threadID,
			"comment_id": resolvedCommentID,
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

func resolveListFormat(raw string, jsonFlag bool) (string, error) {
	if jsonFlag {
		return "json", nil
	}
	format := strings.ToLower(strings.TrimSpace(raw))
	if format == "" {
		format = "auto"
	}
	switch format {
	case "auto":
		if stdoutIsTTY() {
			return "table", nil
		}
		return "json", nil
	case "table", "json", "jsonl":
		return format, nil
	default:
		return "", errors.New(`--format must be "auto", "table", "json", or "jsonl"`)
	}
}

func stdoutIsTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func parseCommentReference(raw string) (int64, error) {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return 0, errors.New("comment reference is empty")
	}
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil && id > 0 {
		return id, nil
	}

	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		u, err := url.Parse(ref)
		if err != nil {
			return 0, err
		}
		if anchor := strings.TrimSpace(u.Fragment); anchor != "" {
			switch {
			case strings.HasPrefix(anchor, "discussion_r"):
				return strconv.ParseInt(strings.TrimPrefix(anchor, "discussion_r"), 10, 64)
			case strings.HasPrefix(anchor, "issuecomment-"):
				return strconv.ParseInt(strings.TrimPrefix(anchor, "issuecomment-"), 10, 64)
			}
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 4 {
			last := parts[len(parts)-1]
			prev := parts[len(parts)-2]
			if prev == "comments" && (parts[len(parts)-3] == "pulls" || parts[len(parts)-3] == "issues") {
				return strconv.ParseInt(last, 10, 64)
			}
		}
	}

	return 0, fmt.Errorf("invalid comment reference %q", raw)
}

func findThreadIDByComment(records []threadRecord, commentID int64) (string, error) {
	for _, r := range records {
		if r.CommentID == commentID {
			return r.ThreadID, nil
		}
		for _, c := range r.AllComments {
			if c.DatabaseID == commentID {
				return r.ThreadID, nil
			}
		}
	}
	return "", fmt.Errorf("review thread not found for comment %d", commentID)
}

func postTopLevelComment(client *api.RESTClient, owner, repo string, pr int, text string, jsonOut bool) error {
	path := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, pr)
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
			"action":     "comment",
			"type":       "top-level",
			"pr":         pr,
			"comment_id": out.ID,
			"url":        out.HTMLURL,
		})
	}
	fmt.Printf("commented: pr=%d comment=%d %s\n", pr, out.ID, out.HTMLURL)
	return nil
}

func buildInlineCommentRequest(
	client *api.RESTClient,
	owner, repo string,
	pr int,
	text, pathArg, commitID, subjectType string,
	line int,
	side string,
	startLine int,
	startSide string,
) (inlineCommentRequest, error) {
	req := inlineCommentRequest{
		Body: text,
		Path: strings.TrimSpace(pathArg),
	}
	if req.Path == "" {
		return inlineCommentRequest{}, errors.New("--path is required for inline review comments")
	}

	normalizedSubject := strings.ToLower(strings.TrimSpace(subjectType))
	if normalizedSubject != "" && normalizedSubject != "file" {
		return inlineCommentRequest{}, errors.New(`--subject must be "file" when provided`)
	}

	if strings.TrimSpace(commitID) == "" {
		headSHA, err := fetchPullHeadSHA(client, owner, repo, pr)
		if err != nil {
			return inlineCommentRequest{}, err
		}
		req.CommitID = headSHA
	} else {
		req.CommitID = strings.TrimSpace(commitID)
	}

	if normalizedSubject == "file" {
		if line > 0 || side != "" || startLine > 0 || startSide != "" {
			return inlineCommentRequest{}, errors.New("--subject file cannot be combined with --line, --side, --start-line, or --start-side")
		}
		req.SubjectType = "file"
		return req, nil
	}

	if line <= 0 {
		return inlineCommentRequest{}, errors.New("inline review comments require --line unless using --subject file")
	}
	normalizedSide, err := normalizeDiffSide(side, "--side")
	if err != nil {
		return inlineCommentRequest{}, err
	}
	req.Line = line
	req.Side = normalizedSide

	if startLine > 0 || strings.TrimSpace(startSide) != "" {
		if startLine <= 0 {
			return inlineCommentRequest{}, errors.New("--start-line is required when using --start-side")
		}
		normalizedStartSide, err := normalizeDiffSide(startSide, "--start-side")
		if err != nil {
			return inlineCommentRequest{}, err
		}
		req.StartLine = startLine
		req.StartSide = normalizedStartSide
	}

	return req, nil
}

func postInlineComment(client *api.RESTClient, owner, repo string, pr int, req inlineCommentRequest, jsonOut bool) error {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, pr)
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	var out reviewCommentResponse
	if err := client.Post(path, bytes.NewReader(bodyBytes), &out); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		payload := map[string]interface{}{
			"action":     "comment",
			"type":       "inline",
			"pr":         pr,
			"comment_id": out.ID,
			"url":        out.HTMLURL,
			"path":       out.Path,
			"commit_id":  out.CommitID,
		}
		if out.Line != nil {
			payload["line"] = *out.Line
		}
		if out.StartLine != nil {
			payload["start_line"] = *out.StartLine
		}
		if out.Side != "" {
			payload["side"] = out.Side
		}
		if out.StartSide != "" {
			payload["start_side"] = out.StartSide
		}
		if out.SubjectType != "" {
			payload["subject_type"] = out.SubjectType
		}
		return enc.Encode(payload)
	}
	fmt.Printf("commented-inline: pr=%d comment=%d %s\n", pr, out.ID, out.HTMLURL)
	return nil
}

func editComment(client *api.RESTClient, owner, repo string, commentID int64, rawType string, text string) (string, commentResponse, error) {
	commentType, err := normalizeCommentType(rawType)
	if err != nil {
		return "", commentResponse{}, err
	}
	req := map[string]string{"body": text}

	for _, endpoint := range commentEndpoints(owner, repo, commentID, commentType) {
		var out commentResponse
		err := doRESTJSON(client, "PATCH", endpoint.path, req, &out)
		if err == nil {
			return endpoint.kind, out, nil
		}
		if commentType == "auto" && isNotFoundError(err) {
			continue
		}
		return "", commentResponse{}, err
	}
	return "", commentResponse{}, fmt.Errorf("comment %d not found in %s", commentID, owner+"/"+repo)
}

func deleteComment(client *api.RESTClient, owner, repo string, commentID int64, rawType string) (string, error) {
	commentType, err := normalizeCommentType(rawType)
	if err != nil {
		return "", err
	}

	for _, endpoint := range commentEndpoints(owner, repo, commentID, commentType) {
		err := doRESTJSON(client, "DELETE", endpoint.path, nil, nil)
		if err == nil {
			return endpoint.kind, nil
		}
		if commentType == "auto" && isNotFoundError(err) {
			continue
		}
		return "", err
	}
	return "", fmt.Errorf("comment %d not found in %s", commentID, owner+"/"+repo)
}

func fetchPullHeadSHA(client *api.RESTClient, owner, repo string, pr int) (string, error) {
	var out pullRequestResponse
	if err := client.Get(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, pr), &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Head.SHA) == "" {
		return "", errors.New("pull request head SHA not found")
	}
	return strings.TrimSpace(out.Head.SHA), nil
}

func normalizeDiffSide(raw, flagName string) (string, error) {
	side := strings.ToUpper(strings.TrimSpace(raw))
	switch side {
	case "LEFT", "RIGHT":
		return side, nil
	case "":
		return "", fmt.Errorf("%s is required", flagName)
	default:
		return "", fmt.Errorf("%s must be LEFT or RIGHT", flagName)
	}
}

type commentEndpoint struct {
	kind string
	path string
}

func commentEndpoints(owner, repo string, commentID int64, commentType string) []commentEndpoint {
	review := commentEndpoint{
		kind: "review",
		path: fmt.Sprintf("repos/%s/%s/pulls/comments/%d", owner, repo, commentID),
	}
	topLevel := commentEndpoint{
		kind: "top-level",
		path: fmt.Sprintf("repos/%s/%s/issues/comments/%d", owner, repo, commentID),
	}

	switch commentType {
	case "review":
		return []commentEndpoint{review}
	case "top-level":
		return []commentEndpoint{topLevel}
	default:
		return []commentEndpoint{review, topLevel}
	}
}

func normalizeCommentType(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "auto", nil
	case "review", "inline":
		return "review", nil
	case "top-level", "top", "issue", "conversation":
		return "top-level", nil
	default:
		return "", errors.New(`--type must be "auto", "review", or "top-level"`)
	}
}

func normalizeIssueCloseReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	switch reason {
	case "":
		return "", nil
	case "completed", "not_planned":
		return reason, nil
	default:
		return "", errors.New(`--reason must be "completed" or "not_planned"`)
	}
}

func normalizeMergeMethod(raw string) (string, error) {
	method := strings.ToLower(strings.TrimSpace(raw))
	switch method {
	case "", "squash":
		return "squash", nil
	case "merge", "rebase":
		return method, nil
	default:
		return "", errors.New(`--method must be "merge", "squash", or "rebase"`)
	}
}

func doRESTJSON(client *api.RESTClient, method, path string, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(bodyBytes)
	}

	resp, err := client.Request(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(respBody))
		if message == "" {
			message = resp.Status
		}
		return &restAPIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("GitHub API %s %s failed: HTTP %d: %s", method, path, resp.StatusCode, message),
		}
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}
	return nil
}

func isNotFoundError(err error) bool {
	var apiErr *restAPIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return true
	}
	var ghErr *api.HTTPError
	return errors.As(err, &ghErr) && ghErr.StatusCode == 404
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

func currentGitBranch() (string, error) {
	branch, err := runGit("branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", errors.New("could not determine current git branch; pass --head explicitly")
	}
	return branch, nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
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
	sort.SliceStable(records, func(i, j int) bool {
		primaryCmp, tieCmp := compareThreadRecords(records[i], records[j], key)
		if primaryCmp != 0 {
			if desc {
				return primaryCmp > 0
			}
			return primaryCmp < 0
		}
		return tieCmp < 0
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

func compareThreadRecords(a, b threadRecord, key string) (int, int) {
	var primaryCmp int
	switch key {
	case "created":
		primaryCmp = compareTimes(a.CreatedAt, b.CreatedAt)
	case "path":
		primaryCmp = strings.Compare(a.Path, b.Path)
		if primaryCmp == 0 {
			primaryCmp = compareInts(a.Line, b.Line)
		}
	case "line":
		primaryCmp = strings.Compare(a.Path, b.Path)
		if primaryCmp == 0 {
			primaryCmp = compareInts(a.Line, b.Line)
		}
	case "author":
		primaryCmp = strings.Compare(strings.ToLower(a.Author), strings.ToLower(b.Author))
	case "severity":
		primaryCmp = compareInts(severityRank(b.Severity), severityRank(a.Severity))
	default:
		primaryCmp = compareTimes(a.UpdatedAt, b.UpdatedAt)
	}
	if primaryCmp != 0 {
		return primaryCmp, compareThreadRecordTieBreakers(a, b)
	}
	return 0, compareThreadRecordTieBreakers(a, b)
}

func compareThreadRecordTieBreakers(a, b threadRecord) int {
	var cmp int
	if cmp = compareTimes(a.UpdatedAt, b.UpdatedAt); cmp != 0 {
		return cmp
	}
	if cmp = compareTimes(a.CreatedAt, b.CreatedAt); cmp != 0 {
		return cmp
	}
	if cmp = strings.Compare(a.Path, b.Path); cmp != 0 {
		return cmp
	}
	if cmp = compareInts(a.Line, b.Line); cmp != 0 {
		return cmp
	}
	if cmp = strings.Compare(strings.ToLower(a.Author), strings.ToLower(b.Author)); cmp != 0 {
		return cmp
	}
	if cmp = strings.Compare(a.ThreadID, b.ThreadID); cmp != 0 {
		return cmp
	}
	return compareInts64(a.CommentID, b.CommentID)
}

func compareTimes(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}

func compareInts(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareInts64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func printTable(records []threadRecord) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "THREAD_ID\tCOMMENT_ID\tSTATE\tOUTDATED\tFILE\tAUTHOR\tUPDATED\tSEVERITY\tSUMMARY")
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

func printJSONL(records []threadRecord) error {
	enc := json.NewEncoder(os.Stdout)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

func printStats(w io.Writer, records []threadRecord) {
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
	fmt.Fprintf(w, "stats: total=%d\n", len(records))
	fmt.Fprintf(w, "  by_severity: %s\n", compactMap(bySeverity))
	fmt.Fprintf(w, "  by_author:   %s\n", compactMap(byAuthor))
	fmt.Fprintf(w, "  by_path:     %s\n", compactMap(byPath))
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
