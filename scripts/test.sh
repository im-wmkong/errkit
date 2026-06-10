#!/usr/bin/env bash
# 遍历仓库内所有 go.mod 并跑 go vet / build / test, 复刻 CI 的多 module 行为。
#
# 用法:
#   scripts/test.sh                   # 本地跑, 输出干净
#   scripts/test.sh --group           # CI 模式, 在每个 module 前后发射 ::group::
#   scripts/test.sh -race             # 透传 go test 参数
#   scripts/test.sh --group -race     # 二者可同时
#
# 设计取舍:
#   - 不引入 Makefile / go workspace: 子 module 各自的 require 版本是独立契约,
#     workspace 模式会让 require 在不同子 module 间互相干扰。
#   - --group 仅写 stdout 标记, GitHub Actions 会自动折叠; 本地终端无影响。
#   - 兼容 macOS 自带的 bash 3.2 (无 mapfile / 不依赖 [[ -v ])。

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

# 解析自有标记: --group 不透传给 go; 其他参数透传给 go test。
group=0
extra_args=()
for arg in "$@"; do
  case "$arg" in
    --group) group=1 ;;
    *) extra_args+=("$arg") ;;
  esac
done

begin_group() {
  if [ "$group" = "1" ]; then
    echo "::group::$1"
  else
    echo
    echo "==> $1"
  fi
}

end_group() {
  if [ "$group" = "1" ]; then
    echo "::endgroup::"
  fi
}

# 找出所有 go.mod, 按路径排序; 排除 .git 与 vendor。
# 用 while read 而非 mapfile, 兼容 macOS 自带的 bash 3.2。
mods=()
while IFS= read -r line; do
  mods+=("$line")
done < <(find . -name go.mod -not -path "./.git/*" -not -path "*/vendor/*" -print | sort)

failed=()

for mod in "${mods[@]}"; do
  dir=$(dirname "$mod")
  begin_group "$dir"
  ok=1
  if ! (cd "$dir" && go vet ./...); then
    failed+=("$dir (vet)")
    ok=0
  fi
  if [ "$ok" = "1" ] && ! (cd "$dir" && go build ./...); then
    failed+=("$dir (build)")
    ok=0
  fi
  if [ "$ok" = "1" ] && ! (cd "$dir" && go test ./... -count=1 ${extra_args[@]+"${extra_args[@]}"}); then
    failed+=("$dir (test)")
    ok=0
  fi
  end_group
done

echo
if [ ${#failed[@]} -gt 0 ]; then
  echo "FAILED:"
  printf '  - %s\n' "${failed[@]}"
  exit 1
fi
echo "All modules passed."
