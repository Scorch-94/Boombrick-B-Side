package version

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"testing"
)

// forkRepo is where Boombrick B-Side is released.
const forkRepo = "Scorch-94/Boombrick-B-Side"

// Settings must check this fork's releases for updates, never upstream's:
// upstream's zips aren't named for the fork, so an update would stall, and
// one that weren't could swap B-Side for the original. The app's checker
// takes version.Repo, set by the build; this reads cmd/boombrick/main.go to
// see that it still does, as a merge or a revert can quietly put a literal
// back. It runs in the release workflow, before the build.
func TestUpdatesCheckThisFork(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "../../cmd/boombrick/main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var checkers int
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isSelector(call.Fun, "release", "NewChecker") || len(call.Args) < 2 {
			return true
		}
		repo := call.Args[1]
		if lit, ok := repo.(*ast.BasicLit); ok {
			s, _ := strconv.Unquote(lit.Value)
			if s == "yt-dlp/yt-dlp" { // the downloader's own releases
				return true
			}
			t.Errorf("%s: release.NewChecker checks %s; the app's own updates must check version.Repo",
				fset.Position(call.Pos()), lit.Value)
			return true
		}
		if !isSelector(repo, "version", "Repo") {
			t.Errorf("%s: the app's release checker should take version.Repo", fset.Position(call.Pos()))
		}
		checkers++
		return true
	})
	if checkers != 1 {
		t.Errorf("found %d release checkers taking version.Repo, want 1", checkers)
	}
}

// Without the build's setting, Repo and the Makefile's default both name
// this fork.
func TestDefaultRepoIsThisFork(t *testing.T) {
	if Repo != forkRepo {
		t.Errorf("Repo is %q, want %q", Repo, forkRepo)
	}
	mk, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^REPO\s*\?=\s*\$\(or \$\(GITHUB_REPOSITORY\),([^)]+)\)`).FindSubmatch(mk)
	if m == nil || string(m[1]) != forkRepo {
		t.Errorf("the Makefile's REPO default isn't %s", forkRepo)
	}
}

func isSelector(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}
