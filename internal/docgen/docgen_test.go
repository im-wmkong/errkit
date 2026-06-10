package docgen

import (
	"bytes"
	"go/token"
	"strings"
	"testing"

	"github.com/im-wmkong/errkind/internal/scan"
)

func sample() []scan.Definition {
	return []scan.Definition{
		{Code: 20002, Name: "order_invalid", Message: "invalid", Pos: token.Position{Filename: "order.go", Line: 8, Column: 1}},
		{Code: 10001, Name: "user_not_found", Message: "not found", Pos: token.Position{Filename: "user.go", Line: 12, Column: 1}},
	}
}

func TestBuild_SortByCode(t *testing.T) {
	got := Build(sample())
	if len(got) != 2 || got[0].Code != 10001 || got[1].Code != 20002 {
		t.Fatalf("not sorted by code: %+v", got)
	}
}

func TestRenderMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderMarkdown(&buf, Build(sample())); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"# Error Codes",
		"| 10001 | `user_not_found` | not found |",
		"| 20002 | `order_invalid` | invalid |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderJSON(&buf, Build(sample())); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"code": 10001`,
		`"name": "user_not_found"`,
		`"source": "user.go:12:1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestMdEscape(t *testing.T) {
	if got := mdEscape("a|b\nc"); got != `a\|b c` {
		t.Errorf("mdEscape = %q", got)
	}
}
