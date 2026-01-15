#!/bin/bash
# 从 npm 切换到 pnpm 的迁移脚本

echo "==> 从 npm 切换到 pnpm..."

# 检查 pnpm 是否已安装
if ! command -v pnpm &> /dev/null; then
    echo "==> pnpm 未安装，正在安装..."
    npm install -g pnpm
fi

echo "==> 清理 npm 产物..."
rm -rf node_modules package-lock.json

echo "==> 使用 pnpm 安装依赖..."
pnpm install

echo "==> 迁移完成！"
echo ""
echo "现在可以使用 pnpm 命令："
echo "  pnpm run dev    # 开发模式"
echo "  pnpm run build  # 构建"
echo "  pnpm add <pkg>  # 添加包"
