// Command errkind 提供错误码工程相关子命令; 当前实现 doc 子命令用于生成错误码文档。
//
// 用法:
//
//	errkind doc [-format=md|json] [path ...]
//
// 不传路径时扫描当前目录。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/im-wmkong/errkind/internal/docgen"
	"github.com/im-wmkong/errkind/internal/scan"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "doc":
		runDoc(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "errkind: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: errkind <command> [arguments]

Commands:
  doc    generate error code documentation from source

Run "errkind doc -h" for command-specific flags.`)
}

func runDoc(args []string) {
	fs := flag.NewFlagSet("doc", flag.ExitOnError)
	format := fs.String("format", "md", "output format: md | json")
	out := fs.String("o", "-", "output file ('-' for stdout)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	dirs := fs.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	dirs = expandEllipsis(dirs)

	defs, scanErrs := scan.ScanDirs(dirs)
	for _, e := range scanErrs {
		fmt.Fprintln(os.Stderr, "errkind doc:", e)
	}
	entries := docgen.Build(defs)

	w := os.Stdout
	if *out != "-" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "errkind doc:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	var renderErr error
	switch *format {
	case "md", "markdown":
		renderErr = docgen.RenderMarkdown(w, entries)
	case "json":
		renderErr = docgen.RenderJSON(w, entries)
	default:
		fmt.Fprintf(os.Stderr, "errkind doc: unknown format %q (want md|json)\n", *format)
		os.Exit(2)
	}
	if renderErr != nil {
		fmt.Fprintln(os.Stderr, "errkind doc:", renderErr)
		os.Exit(1)
	}
}

// expandEllipsis 把 Go 风格的 "dir/..." 归一为 "dir"; 扫描器本身递归。
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
