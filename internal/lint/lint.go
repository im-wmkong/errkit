// Package lint 基于 internal/scan 的扫描结果, 检测 (code, name) 冲突与显然错误。
package lint

import (
	"fmt"
	"go/token"
	"sort"

	"github.com/im-wmkong/errkind/internal/scan"
)

// Issue 表示一条 lint 发现; 多重定义时一个 code/name 会产生多条互相引用的 Issue。
type Issue struct {
	Pos     token.Position
	Message string
}

// Check 在已扫描的 Definition 列表上执行规则, 返回按位置排序的 Issue。
func Check(defs []scan.Definition) []Issue {
	var issues []Issue

	byCode := map[uint32][]scan.Definition{}
	byName := map[string][]scan.Definition{}
	for _, d := range defs {
		if d.Name == "" {
			issues = append(issues, Issue{
				Pos:     d.Pos,
				Message: "errkind.Define: name must not be empty",
			})
			continue
		}
		byCode[d.Code] = append(byCode[d.Code], d)
		byName[d.Name] = append(byName[d.Name], d)
	}

	for code, ds := range byCode {
		if len(ds) < 2 {
			continue
		}
		for _, d := range ds {
			others := otherLocations(ds, d.Pos)
			issues = append(issues, Issue{
				Pos: d.Pos,
				Message: fmt.Sprintf(
					"errkind.Define: duplicate code %d (name=%q); also defined at %s",
					code, d.Name, others),
			})
		}
	}

	for name, ds := range byName {
		if len(ds) < 2 {
			continue
		}
		for _, d := range ds {
			others := otherLocations(ds, d.Pos)
			issues = append(issues, Issue{
				Pos: d.Pos,
				Message: fmt.Sprintf(
					"errkind.Define: duplicate name %q (code=%d); also defined at %s",
					name, d.Code, others),
			})
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Pos.Filename != issues[j].Pos.Filename {
			return issues[i].Pos.Filename < issues[j].Pos.Filename
		}
		if issues[i].Pos.Line != issues[j].Pos.Line {
			return issues[i].Pos.Line < issues[j].Pos.Line
		}
		return issues[i].Message < issues[j].Message
	})
	return issues
}

func otherLocations(ds []scan.Definition, self token.Position) string {
	var locs []string
	for _, d := range ds {
		if d.Pos == self {
			continue
		}
		locs = append(locs, d.Pos.String())
	}
	sort.Strings(locs)
	if len(locs) == 0 {
		return "(none)"
	}
	out := locs[0]
	for _, l := range locs[1:] {
		out += ", " + l
	}
	return out
}
