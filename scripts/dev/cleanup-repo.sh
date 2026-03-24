#!/bin/bash

# 仓库清理脚本
# 删除不需要上传到 GitHub 的临时文件

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${YELLOW}开始清理仓库...${NC}"
echo ""

# 删除编译产物
echo "清理编译产物..."
rm -f v vpanel vpanel-agent agent
rm -f *.exe *.dll *.so *.dylib
rm -rf build/
echo -e "${GREEN}✓ 编译产物已清理${NC}"

# 删除系统文件
echo "清理系统文件..."
find . -name ".DS_Store" -type f -delete
find . -name "Thumbs.db" -type f -delete
find . -name "*.tmp" -type f -delete
find . -name "*.temp" -type f -delete
echo -e "${GREEN}✓ 系统文件已清理${NC}"

# 删除日志文件
echo "清理日志文件..."
rm -rf logs/
find . -name "*.log" -type f -delete
echo -e "${GREEN}✓ 日志文件已清理${NC}"

# 删除备份文件
echo "清理备份文件..."
find . -name "*.backup" -type f -delete
find . -name "*.bak" -type f -delete
find . -name "*.old" -type f -delete
rm -rf backups/
echo -e "${GREEN}✓ 备份文件已清理${NC}"

# 删除临时开发文档
echo "清理临时文档..."
cd Docs
rm -f *-fix*.md
rm -f *-summary*.md
rm -f dev-notes*.md
rm -f immediate-*.md
rm -f deep-check*.md
rm -f error-fix*.md
rm -f fix-completed.md
rm -f task-completed*.md
rm -f troubleshooting-guide.md
rm -f frontend-debug-guide.md
rm -f quick-fix-reference.md
rm -f stats-fix-quick-reference.md
rm -f stats-loading-fix.md
rm -f ip-restriction-*.md
rm -f macos-*.md
rm -f portal-admin-integration-test.md
rm -f puppeteer-testing.md
rm -f routing-changes.md
rm -f admin-redirect.md
rm -f api-database-fix.md
rm -f database-migration-guide.md
rm -f database-recommendation.md
rm -f 用户通知*.md
rm -f 解决方案*.md
rm -f FINAL-FIX-SUMMARY.md
rm -f STATS-FIX-README.md
cd ..
echo -e "${GREEN}✓ 临时文档已清理${NC}"

# 删除空的 Docs 文件夹（如果有重复）
echo "检查重复文件夹..."
if [ -d " Docs" ]; then
    rm -rf " Docs"
    echo -e "${GREEN}✓ 删除了重复的 Docs 文件夹${NC}"
fi

# 清理 Git 缓存
echo ""
echo -e "${YELLOW}清理 Git 缓存...${NC}"
git rm -r --cached . > /dev/null 2>&1 || true
git add .
echo -e "${GREEN}✓ Git 缓存已清理${NC}"

echo ""
echo -e "${GREEN}清理完成！${NC}"
echo ""
echo "下一步操作:"
echo "1. 检查更改: git status"
echo "2. 提交更改: git commit -m 'chore: cleanup repository'"
echo "3. 推送到 GitHub: git push"
