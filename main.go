package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil || n <= 0 {
		exit(fmt.Sprintf("expected a positive PR number, got %q", os.Args[1]))
	}

	owner, name, err := currentRepo()
	if err != nil {
		exit("not inside a git repository (or no remote configured)")
	}

	client, err := api.DefaultRESTClient()
	if err != nil {
		exit(err.Error())
	}

	var pr struct {
		Head struct {
			Ref   string `json:"ref"`
			Repo  struct {
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
			} `json:"repo"`
		} `json:"head"`
	}
	if err := client.Get(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, name, n), &pr); err != nil {
		exit(fmt.Sprintf("fetching PR #%d: %v", n, err))
	}
	if pr.Head.Ref == "" {
		exit(fmt.Sprintf("no head branch returned for PR #%d", n))
	}

	branchOwner := owner
	if pr.Head.Repo.Owner.Login != "" {
		branchOwner = pr.Head.Repo.Owner.Login
	}
	qualified := fmt.Sprintf("refs/heads/%s:%s", branchOwner, pr.Head.Ref)
	plain := fmt.Sprintf("refs/heads/%s", pr.Head.Ref)

	for _, wt := range worktrees() {
		if wt.Branch == qualified || wt.Branch == plain {
			fmt.Println(wt.Path)
			return
		}
	}

	exit(fmt.Sprintf("the branch for PR #%d (%s) is not checked out in any local worktree", n, pr.Head.Ref))
}

type worktree struct {
	Path   string
	Branch string
}

func worktrees() []worktree {
	out, err := runGit("worktree", "list", "--porcelain")
	if err != nil {
		exit("git worktree list: " + err.Error())
	}
	var trees []worktree
	var cur worktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(line, "branch ")
		case line == "" && cur.Path != "":
			trees = append(trees, cur)
			cur = worktree{}
		}
	}
	if cur.Path != "" {
		trees = append(trees, cur)
	}
	return trees
}

func currentRepo() (string, string, error) {
	out, err := runGit("remote", "get-url", "origin")
	if err != nil || out == "" {
		return "", "", err
	}
	return parseGitURL(out)
}

func parseGitURL(u string) (string, string, error) {
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimSuffix(u, "/")
	var path string
	switch {
	case strings.Contains(u, "://"): // https://host/owner/repo
		parts := strings.SplitN(u, "://", 2)
		path = parts[1]
	case strings.HasPrefix(u, "git@"): // git@host:owner/repo
		path = strings.SplitN(u, ":", 2)[1]
	default:
		return "", "", fmt.Errorf("unrecognised remote URL %q", u)
	}
	segs := strings.Split(path, "/")
	if len(segs) < 2 {
		return "", "", fmt.Errorf("unrecognised remote URL %q", u)
	}
	return segs[len(segs)-2], segs[len(segs)-1], nil
}

func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gh cd pr <number>")
	fmt.Fprintln(os.Stderr, "Prints the worktree path of an already-checked-out local PR branch.")
	fmt.Fprintln(os.Stderr, "Use with cd: cd $(gh cd pr <number>)")
	os.Exit(1)
}

func exit(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	os.Exit(1)
}
