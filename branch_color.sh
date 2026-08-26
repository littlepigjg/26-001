#!/bin/bash
set -e

# ============================================
#  Git Worktree 分支创建脚本 - 第二级
#  在 XX-CC-bugN 分支上运行，创建 XX-CC-bugN-red 和 XX-CC-bugN-green
#  用法：在 bug 分支目录运行，自动创建两个子分支并推送到远程
# ============================================

if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "[ERROR] 当前目录不是 Git 仓库"
    exit 1
fi

ORIG_DIR="$(pwd)"
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"

# 验证当前分支是 bug 分支
if [[ ! "$CURRENT_BRANCH" =~ -bug[0-9]+$ ]]; then
    echo "[ERROR] 当前分支 '$CURRENT_BRANCH' 不是 bug 分支"
    echo "        请在 XX-CC-bugN 分支上运行此脚本"
    exit 1
fi

BASE_NAME="$CURRENT_BRANCH"
PARENT_DIR="$(dirname "$ORIG_DIR")"

echo "当前分支: $CURRENT_BRANCH"
echo "基础名称: $BASE_NAME"
echo

echo "将基于分支 '$CURRENT_BRANCH' 创建 red 和 green 分支："
echo "============================================================"

SUCCESS=0
FAIL=0

for COLOR in green red; do
    NEW_BRANCH="${BASE_NAME}-${COLOR}"
    NEW_DIR="${PARENT_DIR}/${NEW_BRANCH}"

    if git show-ref --verify --quiet "refs/heads/$NEW_BRANCH" 2>/dev/null; then
        echo "[SKIP] 分支 '$NEW_BRANCH' 已存在，跳过"
        continue
    fi

    if [ -d "$NEW_DIR" ]; then
        echo "[SKIP] 目录 '$NEW_DIR' 已存在，跳过"
        continue
    fi

    echo -n "[CREATE] $NEW_BRANCH ... "
    if git worktree add -b "$NEW_BRANCH" "$NEW_DIR" "$CURRENT_BRANCH"; then
        echo -n "✓ 创建成功, 推送中... "
        if git push origin "$NEW_BRANCH" 2>/dev/null; then
            echo "✓ 推送成功"
            ((SUCCESS++))
        else
            echo "⚠ 推送失败(远程可能不存在)"
            ((SUCCESS++))
        fi
    else
        echo "✗ 创建失败"
        ((FAIL++))
    fi
done

echo "============================================================"
echo "完成！成功: ${SUCCESS}, 失败: ${FAIL}"
echo
echo "📂 目录结构："
for COLOR in green red; do
    NEW_BRANCH="${BASE_NAME}-${COLOR}"
    if git show-ref --verify --quiet "refs/heads/$NEW_BRANCH" 2>/dev/null; then
        echo "   ${PARENT_DIR}/${NEW_BRANCH}/  (分支: $NEW_BRANCH)"
    fi
done

echo
echo "💡 提示：可进入 ${BASE_NAME}-green/ 或 ${BASE_NAME}-red/ 开始开发"
