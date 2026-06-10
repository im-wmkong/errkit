// Package docgen 把扫描得到的 errkind.Define 列表渲染成错误码文档。
//
// 支持 markdown 与 json 两种格式。Markdown 用表格形态, 适合贴 wiki;
// JSON 适合喂给前端 codegen 或 SRE 告警平台。
package docgen

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/im-wmkong/errkind/internal/scan"
)

// Entry 是文档中的单条错误码记录。
type Entry struct {
	Code    uint32 `json:"code"`
	Name    string `json:"name"`
	Message string `json:"message,omitempty"`
	Source  string `json:"source"`
}

// Build 把扫描得到的 Definition 转成按 code 升序排列的 Entry 列表。
func Build(defs []scan.Definition) []Entry {
	out := make([]Entry, 0, len(defs))
	for _, d := range defs {
		out = append(out, Entry{
			Code:    d.Code,
			Name:    d.Name,
			Message: d.Message,
			Source:  d.Pos.String(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// RenderMarkdown 写出 Markdown 表格。
func RenderMarkdown(w io.Writer, entries []Entry) error {
	if _, err := fmt.Fprintln(w, "# Error Codes"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Code | Name | Default Message | Source |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "|---:|---|---|---|"); err != nil {
		return err
	}
	for _, e := range entries {
		_, err := fmt.Fprintf(w, "| %d | `%s` | %s | `%s` |\n",
			e.Code, e.Name, mdEscape(e.Message), e.Source)
		if err != nil {
			return err
		}
	}
	return nil
}

// RenderJSON 写出缩进 2 空格的 JSON 数组。
func RenderJSON(w io.Writer, entries []Entry) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

// mdEscape 处理 message 中的 | 与换行, 避免破坏表格。
func mdEscape(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
