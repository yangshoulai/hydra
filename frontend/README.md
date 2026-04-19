# Hydra Frontend

Hydra 管理后台前端，基于 Vue 3 + TypeScript + Vite + Naive UI。

## 本地开发

```bash
pnpm install
pnpm dev
```

默认开发地址：

- `http://localhost:5173`

## 生产构建

```bash
pnpm build
```

## 目录说明

- `src/pages/`：页面
- `src/components/`：通用组件与业务组件
- `src/services/`：API 请求封装
- `src/stores/`：Pinia 状态管理
- `src/types/`：类型定义
- `src/utils/`：工具函数

## UI 约定

- 整体风格：黑白极简
- 组件库：Naive UI
- 页面容器：优先复用全局 `app-page`、`panel-card`、`table-toolbar`
- 表单提示：优先使用 `placeholder` 与 `.form-hint`，避免恢复 Naive UI 默认 feedback 区域
