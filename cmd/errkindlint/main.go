// Command errkindlint 静态扫描 errkind.Define 调用, 检查 (code, name) 冲突。
//
// 用法:
//
//	errkindlint [-exclude=glob] [path ...]
//
// 不传路径时扫描当前目录; 发现冲突时退出码为 1。
// -exclude 可重复, 用 filepath.Match 风格匹配文件路径 (例: "*/examples/*")。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/im-wmkong/errkind/internal/lint"
	"github.com/im-wmkong/errkind/internal/scan"
)

type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	var excludes stringSlice
	flag.Var(&excludes, "exclude", "glob to skip files (filepath.Match; repeatable)")
	flag.Parse()

	dirs := flag.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	dirs = expandEllipsis(dirs)

	defs, scanErrs := scan.ScanDirs(dirs)
	for _, e := range scanErrs {
		fmt.Fprintln(os.Stderr, "errkindlint:", e)
	}
	defs = filterExcluded(defs, excludes)

	issues := lint.Check(defs)
	for _, is := range issues {
		fmt.Printf("%s: %s\n", is.Pos, is.Message)
	}
	if len(issues) > 0 {
		os.Exit(1)
	}
}

func filterExcluded(defs []scan.Definition, patterns []string) []scan.Definition {
	if len(patterns) == 0 {
		return defs
	}
	out := defs[:0]
	for _, d := range defs {
		skip := false
		for _, p := range patterns {
			if ok, _ := filepath.Match(p, d.Pos.Filename); ok {
				skip = true
				break
			}
			// Match 不跨 / 通配, 这里再做一次 substring 兜底, 让 "examples/" 这种用法直观。
			if strings.Contains(d.Pos.Filename, p) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, d)
		}
	}
	return out
}

// expandEllipsis 把 Go 风格的 "dir/..." 归一为 "dir"; 扫描器本身递归, 直接去掉后缀即可。
func expandEllipsis(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		switch {
		case p == "./..." || p == "...":
			out = append(out, ".")
		case strings.HasSuffix(p, "/..."):
			out = append(out, strings.TrimSuffix(p, "/..."))
		default:
			out = append(out, p)
		}
	}
	return out
}
