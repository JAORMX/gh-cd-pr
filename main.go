package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil || n <= 0 {
		exit(fmt.Sprintf("expected a positive PR number, got %q", os.Args[1]))
	}

	repo, err := repository.Current()
	if err != nil {
		exit("not inside a git repository, no GitHub remote found, and no GH_REPO set\n\nworktree error: " + err.Error())
	}

	client, err := api.DefaultRESTClient()
	if err != nil {
		exit(err.Error())
	}

	var pr struct {
		Head struct {
			Ref  string `json:"ref"`
			Repo struct {
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
			} `json:"repo"`
		} `json:"head"`
	}
	if err := client.Get(fmt.Sprintf("repos/%s/%s/pulls/%d", repo.Owner, repo.Name, n), &pr); err != nil {
		exit(fmt.Sprintf("fetching PR #%d: %v", n, err))
	}
	if pr.Head.Ref == "" {
		exit(fmt.Sprintf("no head branch returned for PR #%d", n))
	}
	ref := pr.Head.Ref

	owner := repo.Owner
	if pr.Head.Repo.Owner.Login != "" {
		owner = pr.Head.Repo.Owner.Login
	}
	qualified := fmt.Sprintf("refs/heads/%s:%s", owner, ref)
	plain := fmt.Sprintf("refs/heads/%s", ref)
	originQualified := fmt.Sprintf("refs/remotes/origin/%s", ref)

	for _, wt := range worktrees() {
		if wt.Branch == qualified || wt.Branch == originQualified || wt.Branch == plain {
			fmt.Println(wt.Path)
			return
		}
	}

	exit(fmt.Sprintf("PR #%d (%s) is not checked out in any local worktree\nhint: git worktree add <path> %s", n, ref, ref))
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

func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gh pwd-pr <number>")
	fmt.Fprintln(os.Stderr, "Prints the worktree path of an already-checked-out local PR branch.")
	fmt.Fprintln(os.Stderr, "Example: cd $(gh pwd-pr 123)")
	os.Exit(1)
}

func exit(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	os.Exit(1)
}
