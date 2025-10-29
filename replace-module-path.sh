#!/usr/bin/env bash

# 批量替换脚本：将模板项目路径替换为正确的模块路径
# 用法: ./replace-module-path.sh

set -euo pipefail

# 定义颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 定义路径
OLD_PATH="github.com/bionicotaku/lingo-services-catalog"
NEW_PATH="github.com/bionicotaku/lingo-services-profile"
WORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${YELLOW}=== 批量替换模块路径 ===${NC}"
echo "工作目录: ${WORK_DIR}"
echo "旧路径: ${OLD_PATH}"
echo "新路径: ${NEW_PATH}"
echo ""

# 检查是否在正确的目录
if [[ ! -f "${WORK_DIR}/go.mod" ]]; then
    echo -e "${RED}错误: 未找到 go.mod 文件。请确保在 services-profile 目录下运行此脚本。${NC}"
    exit 1
fi

# 1. 搜索包含旧路径的文件
echo -e "${YELLOW}步骤 1: 搜索包含旧路径的文件...${NC}"
MATCHED_FILES=$(grep -rl "${OLD_PATH}" . \
    --include="*.go" \
    --include="*.proto" \
    --include="*.yaml" \
    --include="go.mod" \
    --exclude-dir=".git" \
    --exclude-dir="vendor" \
    2>/dev/null || true)

if [[ -z "${MATCHED_FILES}" ]]; then
    echo -e "${GREEN}✓ 未找到包含旧路径的文件，可能已经替换完成。${NC}"
    exit 0
fi

FILE_COUNT=$(echo "${MATCHED_FILES}" | wc -l | tr -d ' ')
echo -e "${GREEN}找到 ${FILE_COUNT} 个文件包含旧路径${NC}"
echo ""

# 2. 执行批量替换
echo -e "${YELLOW}步骤 2: 执行批量替换...${NC}"

# 根据操作系统选择合适的 sed 命令
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    SED_CMD="sed -i ''"
else
    # Linux
    SED_CMD="sed -i"
fi

REPLACED_COUNT=0
while IFS= read -r file; do
    if [[ -f "${file}" ]]; then
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s|${OLD_PATH}|${NEW_PATH}|g" "${file}"
        else
            sed -i "s|${OLD_PATH}|${NEW_PATH}|g" "${file}"
        fi
        echo "  ✓ ${file}"
        ((REPLACED_COUNT++))
    fi
done <<< "${MATCHED_FILES}"

echo -e "${GREEN}✓ 已替换 ${REPLACED_COUNT} 个文件${NC}"
echo ""

# 3. 运行 go mod tidy
echo -e "${YELLOW}步骤 3: 运行 go mod tidy...${NC}"
cd "${WORK_DIR}"
if env GOWORK=off go mod tidy; then
    echo -e "${GREEN}✓ go mod tidy 完成${NC}"
else
    echo -e "${RED}✗ go mod tidy 失败${NC}"
    exit 1
fi
echo ""

# 4. 运行代码格式化
echo -e "${YELLOW}步骤 4: 运行代码格式化...${NC}"

# gofumpt
if command -v gofumpt &> /dev/null; then
    if env GOWORK=off gofumpt -w .; then
        echo -e "${GREEN}✓ gofumpt 格式化完成${NC}"
    else
        echo -e "${RED}✗ gofumpt 格式化失败${NC}"
    fi
else
    echo -e "${YELLOW}⚠ gofumpt 未安装，跳过${NC}"
fi

# goimports
if command -v goimports &> /dev/null; then
    if env GOWORK=off goimports -w .; then
        echo -e "${GREEN}✓ goimports 格式化完成${NC}"
    else
        echo -e "${RED}✗ goimports 格式化失败${NC}"
    fi
else
    echo -e "${YELLOW}⚠ goimports 未安装，跳过${NC}"
fi
echo ""

# 5. 验证构建
echo -e "${YELLOW}步骤 5: 验证构建...${NC}"
if env GOWORK=off go build ./...; then
    echo -e "${GREEN}✓ 构建成功${NC}"
else
    echo -e "${RED}✗ 构建失败，请检查错误信息${NC}"
    exit 1
fi
echo ""

# 6. 显示修改统计
echo -e "${YELLOW}=== 替换完成 ===${NC}"
echo -e "${GREEN}✓ 已替换 ${REPLACED_COUNT} 个文件${NC}"
echo -e "${GREEN}✓ 所有检查通过${NC}"
echo ""
echo "建议的后续步骤:"
echo "  1. 检查 git diff 确认修改正确"
echo "  2. 运行单元测试: env GOWORK=off go test ./..."
echo "  3. 运行静态检查: make lint"
echo ""
