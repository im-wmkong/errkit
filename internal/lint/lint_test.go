package lint

import (
	"go/token"
	"strings"
	"testing"

	"github.com/im-wmkong/errkind/internal/scan"
)

func pos(file string, line int) token.Position {
	return token.Position{Filename: file, Line: line, Column: 1}
}

func TestCheck_NoIssues(t *testing.T) {
	defs := []scan.Definition{
		{Code: 1, Name: "a", Pos: pos("a.go", 1)},
		{Code: 2, Name: "b", Pos: pos("a.go", 2)},
	}
	if got := Check(defs); len(got) != 0 {
		t.Fatalf("expected no issues, got %+v", got)
	}
}

func TestCheck_DuplicateCode(t *testing.T) {
	defs := []scan.Definition{
		{Code: 1, Name: "a", Pos: pos("a.go", 1)},
		{Code: 1, Name: "b", Pos: pos("b.go", 5)},
	}
	got := Check(defs)
	if len(got) != 2 {
		t.Fatalf("want 2 issues, got %d: %+v", len(got), got)
	}
	for _, is := range got {
		if !strings.Contains(is.Message, "duplicate code 1") {
			t.Errorf("missing duplicate-code marker: %q", is.Message)
		}
	}
}

func TestCheck_DuplicateName(t *testing.T) {
	defs := []scan.Definition{
		{Code: 1, Name: "x", Pos: pos("a.go", 1)},
		{Code: 2, Name: "x", Pos: pos("b.go", 5)},
	}
	got := Check(defs)
	if len(got) != 2 {
		t.Fatalf("want 2 issues, got %d", len(got))
	}
	for _, is := range got {
		if !strings.Contains(is.Message, `duplicate name "x"`) {
			t.Errorf("missing duplicate-name marker: %q", is.Message)
		}
	}
}

func TestCheck_EmptyName(t *testing.T) {
	defs := []scan.Definition{
		{Code: 1, Name: "", Pos: pos("a.go", 1)},
	}
	got := Check(defs)
	if len(got) != 1 || !strings.Contains(got[0].Message, "name must not be empty") {
		t.Fatalf("want empty-name issue, got %+v", got)
	}
}
