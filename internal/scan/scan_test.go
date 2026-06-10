package scan

import (
	"path/filepath"
	"sort"
	"testing"
)

func TestScanDirs_Basic(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import "github.com/im-wmkong/errkind"

var (
	A = errkind.Define(10001, "user_not_found", errkind.DefaultMessage("nope"))
	B = errkind.Define(10002, "user_locked")
)
`)
	// 不导入 errkind 的文件应被忽略
	writeGo(t, filepath.Join(dir, "b.go"), `package x

func F() {}
`)
	// _test.go 应被跳过
	writeGo(t, filepath.Join(dir, "c_test.go"), `package x

import "github.com/im-wmkong/errkind"

var Z = errkind.Define(99999, "in_test")
`)

	defs, errs := ScanDirs([]string{dir})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Code < defs[j].Code })

	if len(defs) != 2 {
		t.Fatalf("got %d defs, want 2: %+v", len(defs), defs)
	}
	if defs[0].Code != 10001 || defs[0].Name != "user_not_found" || defs[0].Message != "nope" {
		t.Errorf("def[0] = %+v", defs[0])
	}
	if defs[1].Code != 10002 || defs[1].Name != "user_locked" || defs[1].Message != "" {
		t.Errorf("def[1] = %+v", defs[1])
	}
}

func TestScanDirs_AliasImport(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import ek "github.com/im-wmkong/errkind"

var A = ek.Define(1, "n")
`)
	defs, errs := ScanDirs([]string{dir})
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(defs) != 1 || defs[0].Code != 1 || defs[0].Name != "n" {
		t.Fatalf("defs = %+v", defs)
	}
}

func TestScanDirs_SkipDotImport(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import . "github.com/im-wmkong/errkind"

var A = Define(1, "n")
`)
	defs, _ := ScanDirs([]string{dir})
	if len(defs) != 0 {
		t.Fatalf("dot-import should be ignored, got %+v", defs)
	}
}

func TestScanDirs_NonLiteralIgnored(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import "github.com/im-wmkong/errkind"

const C = 1
var name = "n"
var A = errkind.Define(C, name)
`)
	defs, _ := ScanDirs([]string{dir})
	if len(defs) != 0 {
		t.Fatalf("non-literal args should be ignored, got %+v", defs)
	}
}

func writeGo(t *testing.T, path, content string) {
	t.Helper()
	if err := writeFile(path, []byte(content)); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
