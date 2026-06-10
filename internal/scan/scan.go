// Package scan 提供静态扫描能力, 在源码层面找出对 errkind.Define 的调用,
// 用于 errkindlint (冲突检测) 和 errkind doc (文档生成) 共享同一份解析逻辑。
//
// 限制:
//   - 仅识别字面量参数 (整型 code 与字符串 name); 变量 / 常量引用不识别。
//   - 仅识别包级 errkind.Define 调用 (含 import alias); 不识别 r.Define (Registry 实例方法)。
//   - 跳过 _test.go 与 vendor / testdata 目录。
package scan

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ModulePath 是 errkind 主模块的导入路径; 扫描器只在导入了它的文件里查找 Define。
const ModulePath = "github.com/im-wmkong/errkind"

// Definition 描述一次 Define(code, name, ...) 调用的静态信息。
type Definition struct {
	Code    uint32
	Name    string
	Message string // DefaultMessage("...") 的字面量, 解析不到时为空
	Pos     token.Position
}

// ScanDirs 递归扫描多个目录, 返回找到的 Define 调用与扫描期间的非致命错误。
//
// 解析失败 / 读取失败的单个文件不会中断整体扫描, 错误被收集到第二个返回值。
func ScanDirs(dirs []string) ([]Definition, []error) {
	var defs []Definition
	var errs []error
	fset := token.NewFileSet()
	for _, dir := range dirs {
		ds, es := scanDir(fset, dir)
		defs = append(defs, ds...)
		errs = append(errs, es...)
	}
	return defs, errs
}

func scanDir(fset *token.FileSet, dir string) ([]Definition, []error) {
	var defs []Definition
	var errs []error

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == "vendor" || base == "testdata" || base == "node_modules" {
				return fs.SkipDir
			}
			if path != dir && strings.HasPrefix(base, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		ds, fe := scanFile(fset, path)
		if fe != nil {
			errs = append(errs, fe)
			return nil
		}
		defs = append(defs, ds...)
		return nil
	})
	if walkErr != nil {
		errs = append(errs, walkErr)
	}
	return defs, errs
}

func scanFile(fset *token.FileSet, path string) ([]Definition, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	alias, ok := findImportAlias(f, ModulePath)
	if !ok {
		return nil, nil
	}

	var defs []Definition
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !isErrkindCall(call.Fun, alias, "Define") {
			return true
		}
		d, ok := parseDefineCall(fset, call)
		if !ok {
			return true
		}
		defs = append(defs, d)
		return true
	})
	return defs, nil
}

// findImportAlias 找出当前文件里 errkind 模块的本地名称。
// 匿名 / 点号导入返回 false (无法构造 errkind.Define 调用)。
func findImportAlias(f *ast.File, modulePath string) (string, bool) {
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != modulePath {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				return "", false
			}
			return imp.Name.Name, true
		}
		// 默认包名 = 路径最后一段。
		idx := strings.LastIndex(modulePath, "/")
		return modulePath[idx+1:], true
	}
	return "", false
}

func isErrkindCall(fn ast.Expr, alias, method string) bool {
	sel, ok := fn.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == alias
}

func parseDefineCall(fset *token.FileSet, call *ast.CallExpr) (Definition, bool) {
	if len(call.Args) < 2 {
		return Definition{}, false
	}
	code, ok := parseUintLit(call.Args[0])
	if !ok {
		return Definition{}, false
	}
	name, ok := parseStringLit(call.Args[1])
	if !ok {
		return Definition{}, false
	}
	msg := ""
	for _, arg := range call.Args[2:] {
		if m, ok := parseDefaultMessage(arg); ok {
			msg = m
		}
	}
	return Definition{
		Code:    code,
		Name:    name,
		Message: msg,
		Pos:     fset.Position(call.Pos()),
	}, true
}

func parseUintLit(e ast.Expr) (uint32, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	v, err := strconv.ParseUint(lit.Value, 0, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

func parseStringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// parseDefaultMessage 识别 errkind.DefaultMessage("...") / DefaultMessage("..."), 取出字面量。
func parseDefaultMessage(arg ast.Expr) (string, bool) {
	call, ok := arg.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	var name string
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		name = fn.Sel.Name
	case *ast.Ident:
		name = fn.Name
	}
	if name != "DefaultMessage" || len(call.Args) != 1 {
		return "", false
	}
	return parseStringLit(call.Args[0])
}
