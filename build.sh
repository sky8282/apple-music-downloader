#!/bin/bash

# Apple Music Downloader 编译脚本
# 用途：注入版本号、编译时间和Git提交信息

set -e

# 读取版本号
if [ -f "VERSION" ]; then
    VERSION=$(cat VERSION | tr -d '\n')
else
    VERSION="dev"
fi

# 获取编译时间（北京时间）
BUILD_TIME=$(TZ='Asia/Shanghai' date '+%Y-%m-%d %H:%M:%S CST')

# 获取Git提交哈希（如果在git仓库中）
if git rev-parse --git-dir > /dev/null 2>&1; then
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    
    # 检查是否有未提交的修改
    if ! git diff-index --quiet HEAD -- 2>/dev/null; then
        GIT_COMMIT="${GIT_COMMIT}-dirty"
    fi
    
    VERSION="${VERSION} (${GIT_BRANCH})"
else
    GIT_COMMIT="unknown"
fi

# 输出编译信息
echo "========================================"
echo "🔨 开始编译 Apple Music Downloader"
echo "========================================"
echo "📦 版本号: ${VERSION}"
echo "📅 编译时间: ${BUILD_TIME}"
echo "🔖 Git提交: ${GIT_COMMIT}"
echo "========================================"
echo ""

# 构建ldflags
LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X 'main.Version=${VERSION}'"
LDFLAGS="${LDFLAGS} -X 'main.BuildTime=${BUILD_TIME}'"
LDFLAGS="${LDFLAGS} -X 'main.GitCommit=${GIT_COMMIT}'"

# 编译
echo "⚙️  正在编译..."
go build -ldflags="${LDFLAGS}" -o apple-music-downloader

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ 编译成功！"
    echo ""
    echo "📄 输出文件: apple-music-downloader"
    echo "💾 文件大小: $(du -h apple-music-downloader | cut -f1)"
    echo ""
    echo "运行方式:"
    echo "  ./apple-music-downloader"
    echo ""
else
    echo ""
    echo "❌ 编译失败！"
    exit 1
fi

