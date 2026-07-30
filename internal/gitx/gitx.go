// Package gitx wraps the git plumbing commands git-backlog is built on.
// Every backlog item is real git objects; this package is the only place
// that shells out to git.
package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes a git command and returns trimmed stdout.
func Run(args ...string) (string, error) {
	return RunStdin("", args...)
}

// RunStdin executes a git command, feeding stdin, and returns trimmed stdout.
func RunStdin(stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// InRepo reports whether the current directory is inside a git repository.
func InRepo() bool {
	_, err := Run("rev-parse", "--git-dir")
	return err == nil
}

// HashBlob writes content as a blob and returns its hash.
func HashBlob(content string) (string, error) {
	return RunStdin(content, "hash-object", "-w", "--stdin")
}

// CatBlob returns the content of a blob.
func CatBlob(hash string) (string, error) {
	return Run("cat-file", "-p", hash)
}

// TreeEntry is one named field in an item's snapshot tree.
type TreeEntry struct {
	Name string
	Hash string
}

// MkTree builds a flat tree (mode 100644, type blob) from entries and
// returns its hash. Entries are sorted by name, as git requires.
func MkTree(entries []TreeEntry) (string, error) {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "100644 blob %s\t%s\n", e.Hash, e.Name)
	}
	return RunStdin(b.String(), "mktree")
}

// LsTree reads the flat (non-recursive) entries of a tree.
func LsTree(tree string) ([]TreeEntry, error) {
	out, err := Run("ls-tree", tree)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for _, line := range splitLines(out) {
		// "100644 blob <hash>\t<name>"
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		fields := strings.Fields(line[:tab])
		if len(fields) != 3 {
			continue
		}
		entries = append(entries, TreeEntry{Name: line[tab+1:], Hash: fields[2]})
	}
	return entries, nil
}

// CommitInfo is a parsed commit object.
type CommitInfo struct {
	Hash        string
	Tree        string
	Parents     []string
	AuthorName  string
	AuthorEmail string
	AuthorDate  string // raw "<unix> <tz>" as stored by git
}

// CatCommit reads and parses a commit object.
func CatCommit(hash string) (*CommitInfo, error) {
	out, err := Run("cat-file", "-p", hash)
	if err != nil {
		return nil, err
	}
	ci := &CommitInfo{Hash: hash}
	for _, line := range splitLines(out) {
		if line == "" {
			break // end of headers
		}
		switch {
		case strings.HasPrefix(line, "tree "):
			ci.Tree = strings.TrimPrefix(line, "tree ")
		case strings.HasPrefix(line, "parent "):
			ci.Parents = append(ci.Parents, strings.TrimPrefix(line, "parent "))
		case strings.HasPrefix(line, "author "):
			name, email, date := parseIdentityLine(strings.TrimPrefix(line, "author "))
			ci.AuthorName, ci.AuthorEmail, ci.AuthorDate = name, email, date
		}
	}
	return ci, nil
}

// parseIdentityLine parses "Name <email> <unix> <tz>".
func parseIdentityLine(s string) (name, email, date string) {
	lt := strings.IndexByte(s, '<')
	gt := strings.IndexByte(s, '>')
	if lt < 0 || gt < 0 || gt < lt {
		return s, "", ""
	}
	name = strings.TrimSpace(s[:lt])
	email = s[lt+1 : gt]
	date = strings.TrimSpace(s[gt+1:])
	return
}

// CommitTree creates a commit object and returns its hash.
func CommitTree(tree string, parents []string, message string) (string, error) {
	args := []string{"commit-tree", tree}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	args = append(args, "-m", message)
	return Run(args...)
}

// UpdateRef sets ref to newHash. If oldHash is non-empty, git enforces a
// compare-and-swap against the ref's current value.
func UpdateRef(ref, newHash, oldHash string) error {
	args := []string{"update-ref", ref, newHash}
	if oldHash != "" {
		args = append(args, oldHash)
	}
	_, err := Run(args...)
	return err
}

// RefLine is one result row from ForEachRef.
type RefLine struct {
	Ref  string
	Hash string
}

// ForEachRef lists refs under pattern with their current object hash.
func ForEachRef(pattern string) ([]RefLine, error) {
	out, err := Run("for-each-ref", "--format=%(refname) %(objectname)", pattern)
	if err != nil {
		return nil, err
	}
	var refs []RefLine
	for _, line := range splitLines(out) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		refs = append(refs, RefLine{Ref: fields[0], Hash: fields[1]})
	}
	return refs, nil
}

// RefExists reports whether ref currently resolves to an object.
func RefExists(ref string) bool {
	_, err := Run("rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// ResolveCommit resolves a (possibly abbreviated) commit-ish to a full hash.
func ResolveCommit(rev string) (string, error) {
	return Run("rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
}

// MergeBase returns the best common ancestor of a and b.
func MergeBase(a, b string) (string, error) {
	return Run("merge-base", a, b)
}

// IsAncestor reports whether ancestor is an ancestor of (or equal to) descendant.
func IsAncestor(ancestor, descendant string) bool {
	_, err := Run("merge-base", "--is-ancestor", ancestor, descendant)
	return err == nil
}

// RevListExclude returns commits reachable from tip but not from base,
// oldest-parent-first traversal order reversed to oldest-first.
func RevListExclude(tip, base string) ([]string, error) {
	out, err := Run("rev-list", "--reverse", tip, "^"+base)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// RevListReverse returns all commits reachable from tip, oldest first.
func RevListReverse(tip string) ([]string, error) {
	out, err := Run("rev-list", "--reverse", tip)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// DiffStatus is one changed top-level entry between two trees.
type DiffStatus struct {
	Name    string
	Status  byte // 'A', 'M', or 'D'
	NewHash string
}

// DiffTree reports which top-level entries changed going from oldTree to
// newTree (both commit or tree hashes).
func DiffTree(oldTree, newTree string) ([]DiffStatus, error) {
	out, err := Run("diff-tree", oldTree, newTree)
	if err != nil {
		return nil, err
	}
	var diffs []DiffStatus
	for _, line := range splitLines(out) {
		// ":100644 100644 <old> <new> M\t<name>"
		if !strings.HasPrefix(line, ":") {
			continue
		}
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		fields := strings.Fields(line[:tab])
		if len(fields) != 5 {
			continue
		}
		diffs = append(diffs, DiffStatus{
			Name:    line[tab+1:],
			Status:  fields[4][0],
			NewHash: fields[3],
		})
	}
	return diffs, nil
}

// Config reads a single git config value; ok is false if unset.
func Config(key string) (value string, ok bool) {
	out, err := Run("config", "--get", key)
	if err != nil {
		return "", false
	}
	return out, true
}

// SetConfig sets a git config value in the local repo.
func SetConfig(key, value string) error {
	_, err := Run("config", key, value)
	return err
}

// Remotes lists configured remote names.
func Remotes() ([]string, error) {
	out, err := Run("remote")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return splitLines(out), nil
}

// Push runs a push with an explicit refspec.
func Push(remote, refspec string) error {
	_, err := Run("push", remote, refspec)
	return err
}

// Fetch runs a fetch with an explicit refspec.
func Fetch(remote, refspec string) error {
	_, err := Run("fetch", remote, refspec)
	return err
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
