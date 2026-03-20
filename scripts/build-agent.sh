#!/bin/bash

# Agent 编译脚本
# 用于编译多平台的 Agent 二进制

set -e

VERSION=${VERSION:-"1.0.0"}
BUILD_DIR="build"
AGENT_CMD="./cmd/agent/main.go"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}开始编译 V Panel Agent${NC}"
echo "版本: $VERSION"
echo ""

# 创建构建目录
mkdir -p $BUILD_DIR

# 编译函数
build() {
    local os=$1
    local arch=$2
    local output_name="vpanel-agent-${os}-${arch}"
    
    if [ "$os" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo -e "${YELLOW}编译 ${os}/${arch}...${NC}"
    
    GOOS=$os GOARCH=$arch go build \
        -ldflags "-X main.version=$VERSION -s -w" \
        -o "$BUILD_DIR/$output_name" \
        $AGENT_CMD
    
    if [ $? -eq 0 ]; then
        size=$(ls -lh "$BUILD_DIR/$output_name" | awk '{print $5}')
        echo -e "${GREEN}✓ $output_name ($size)${NC}"
    else
        echo -e "${RED}✗ 编译失败${NC}"
        return 1
    fi
}

# 编译各平台版本
echo "编译 Linux 版本..."
build linux amd64
build linux arm64
build linux arm

echo ""
echo "编译 macOS 版本..."
build darwin amd64
build darwin arm64

echo ""
echo "编译 Windows 版本..."
build windows amd64

echo ""
echo -e "${GREEN}编译完成！${NC}"
echo ""
echo "输出目录: $BUILD_DIR"
ls -lh $BUILD_DIR/

# 创建压缩包
echo ""
echo "创建压缩包..."
cd $BUILD_DIR
for file in vpanel-agent-*; do
    if [ -f "$file" ]; then
        tar -czf "${file}.tar.gz" "$file"
        echo "✓ ${file}.tar.gz"
    fi
done
cd ..

echo ""
echo -e "${GREEN}所有任务完成！${NC}"
