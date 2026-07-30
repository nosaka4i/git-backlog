package gitx

import (
	"os"
	"os/exec"
	"testing"
)

// chdirTempRepo creates a fresh git repo in a temp dir, chdirs the test
// process into it, and restores the original cwd on cleanup. gitx has no
// notion of a repo path — every call operates on the process cwd, same as
// plain git — so tests need a real repo underfoot.
func chdirTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

func TestInRepo(t *testing.T) {
	chdirTempRepo(t)
	if !InRepo() {
		t.Fatal("expected InRepo() true inside a fresh git repo")
	}
}

func TestBlobRoundTrip(t *testing.T) {
	chdirTempRepo(t)
	hash, err := HashBlob("hello world")
	if err != nil {
		t.Fatal(err)
	}
	got, err := CatBlob(hash)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello world" {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestMkTreeLsTree(t *testing.T) {
	chdirTempRepo(t)
	h1, _ := HashBlob("a")
	h2, _ := HashBlob("b")
	tree, err := MkTree([]TreeEntry{{Name: "title", Hash: h1}, {Name: "list", Hash: h2}})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := LsTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.Name] = e.Hash
	}
	if got["title"] != h1 || got["list"] != h2 {
		t.Fatalf("ls-tree mismatch: %+v", got)
	}
}

func TestCommitTreeAndCatCommit(t *testing.T) {
	chdirTempRepo(t)
	h1, _ := HashBlob("hello")
	tree, err := MkTree([]TreeEntry{{Name: "title", Hash: h1}})
	if err != nil {
		t.Fatal(err)
	}
	commit, err := CommitTree(tree, nil, "add: hello")
	if err != nil {
		t.Fatal(err)
	}
	ci, err := CatCommit(commit)
	if err != nil {
		t.Fatal(err)
	}
	if ci.Tree != tree {
		t.Fatalf("tree = %s, want %s", ci.Tree, tree)
	}
	if len(ci.Parents) != 0 {
		t.Fatalf("expected no parents, got %v", ci.Parents)
	}
	if ci.AuthorName != "Test User" || ci.AuthorEmail != "test@example.com" {
		t.Fatalf("author = %s <%s>", ci.AuthorName, ci.AuthorEmail)
	}

	tree2, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}, {Name: "list", Hash: h1}})
	child, err := CommitTree(tree2, []string{commit}, "list: backlog")
	if err != nil {
		t.Fatal(err)
	}
	ci2, err := CatCommit(child)
	if err != nil {
		t.Fatal(err)
	}
	if len(ci2.Parents) != 1 || ci2.Parents[0] != commit {
		t.Fatalf("parents = %v, want [%s]", ci2.Parents, commit)
	}
}

func TestUpdateRefAndForEachRef(t *testing.T) {
	chdirTempRepo(t)
	h1, _ := HashBlob("x")
	tree, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}})
	commit, err := CommitTree(tree, nil, "add: x")
	if err != nil {
		t.Fatal(err)
	}
	ref := "refs/backlog/" + commit
	if err := UpdateRef(ref, commit, ""); err != nil {
		t.Fatal(err)
	}
	if !RefExists(ref) {
		t.Fatal("expected ref to exist after UpdateRef")
	}
	refs, err := ForEachRef("refs/backlog/")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Ref != ref || refs[0].Hash != commit {
		t.Fatalf("for-each-ref = %+v", refs)
	}

	// CAS with a stale old hash must fail.
	if err := UpdateRef(ref, commit, "0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("expected UpdateRef with stale old hash to fail")
	}
}

func TestDiffTree(t *testing.T) {
	chdirTempRepo(t)
	hA, _ := HashBlob("a")
	hB, _ := HashBlob("b")
	t1, _ := MkTree([]TreeEntry{{Name: "title", Hash: hA}, {Name: "priority", Hash: hA}})
	t2, _ := MkTree([]TreeEntry{{Name: "title", Hash: hA}, {Name: "priority", Hash: hB}})
	t3, _ := MkTree([]TreeEntry{{Name: "title", Hash: hA}})

	diffs, err := DiffTree(t1, t2)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || diffs[0].Name != "priority" || diffs[0].Status != 'M' {
		t.Fatalf("modify diff = %+v", diffs)
	}

	diffs, err = DiffTree(t1, t3)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || diffs[0].Name != "priority" || diffs[0].Status != 'D' {
		t.Fatalf("delete diff = %+v", diffs)
	}
}

func TestMergeBaseAndIsAncestor(t *testing.T) {
	chdirTempRepo(t)
	h1, _ := HashBlob("a")
	tree, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}})
	base, _ := CommitTree(tree, nil, "base")

	tree2, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}, {Name: "list", Hash: h1}})
	left, _ := CommitTree(tree2, []string{base}, "left")

	tree3, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}, {Name: "priority", Hash: h1}})
	right, _ := CommitTree(tree3, []string{base}, "right")

	mb, err := MergeBase(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if mb != base {
		t.Fatalf("merge-base = %s, want %s", mb, base)
	}
	if !IsAncestor(base, left) || !IsAncestor(base, right) {
		t.Fatal("expected base to be an ancestor of both branches")
	}
	if IsAncestor(left, right) {
		t.Fatal("left and right diverged; neither should be the other's ancestor")
	}
}

func TestRevListExcludeAndReverse(t *testing.T) {
	chdirTempRepo(t)
	h1, _ := HashBlob("a")
	tree, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}})
	c1, _ := CommitTree(tree, nil, "c1")
	tree2, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}, {Name: "list", Hash: h1}})
	c2, _ := CommitTree(tree2, []string{c1}, "c2")

	all, err := RevListReverse(c2)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0] != c1 || all[1] != c2 {
		t.Fatalf("RevListReverse = %v", all)
	}

	excl, err := RevListExclude(c2, c1)
	if err != nil {
		t.Fatal(err)
	}
	if len(excl) != 1 || excl[0] != c2 {
		t.Fatalf("RevListExclude = %v", excl)
	}
}

func TestConfigSetGet(t *testing.T) {
	chdirTempRepo(t)
	if _, ok := Config("backlog.init"); ok {
		t.Fatal("expected backlog.init unset in a fresh repo")
	}
	if err := SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	v, ok := Config("backlog.init")
	if !ok || v != "true" {
		t.Fatalf("Config = %q, %v", v, ok)
	}
}

func TestResolveCommit(t *testing.T) {
	chdirTempRepo(t)
	h1, _ := HashBlob("a")
	tree, _ := MkTree([]TreeEntry{{Name: "title", Hash: h1}})
	commit, err := CommitTree(tree, nil, "c")
	if err != nil {
		t.Fatal(err)
	}
	full, err := ResolveCommit(commit[:8])
	if err != nil {
		t.Fatal(err)
	}
	if full != commit {
		t.Fatalf("ResolveCommit(%s) = %s, want %s", commit[:8], full, commit)
	}
	if _, err := ResolveCommit("0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("expected error resolving a nonexistent commit")
	}
}

func TestSplitLinesHelper(t *testing.T) {
	if got := splitLines(""); got != nil {
		t.Fatalf("splitLines(\"\") = %v, want nil", got)
	}
	if got := splitLines("a\nb"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("splitLines mismatch: %v", got)
	}
}

func TestMain(m *testing.M) {
	// Guard against a missing git binary producing confusing test failures.
	if _, err := exec.LookPath("git"); err != nil {
		panic("git not found on PATH: " + err.Error())
	}
	os.Exit(m.Run())
}
