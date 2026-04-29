# Hydra 项目指南

面向 AI 代理的项目上下文与约定。

## 项目概述

Hydra 是一个 AI 网关管理平台，结构：
- `backend/` — Go 后端（API / 代理 / 中间件 / 服务 / 数据迁移）
- `frontend/` — Vue 3 + TypeScript + Naive UI 管理控制台
- `deployments/` — 部署脚本
- `data/` — 本地数据

## 前端 UI 约定（frontend/）

UI 框架为 **Naive UI 2.43+**，整体风格黑白极简。下面是几条在本项目里被反复验证过、必须遵守的规则：

### 表单

- **所有表单只用 `placeholder` 进行输入提示**，不在 label 下放描述文字或 tooltip。
  - 原因：Naive UI 的 `n-form-item-feedback-wrapper` 会在校验触发时出现并抢占 14px 高度，引发整页"抖动"。
  - 实现：已在 `src/style.css` 全局隐藏 `feedback-wrapper`（`display: none !important`）。校验失败的反馈通过 toast（`message.error` / `feedback.message?.error`）而非行内文字。
  - **不要**在单个组件里把 feedback-wrapper 恢复显示。
- 额外的字段说明需要显示时，使用 `.form-hint` class（定义在全局 `style.css`）放在输入控件之下作为静态说明。

### 布局

- 页面容器：`div.app-page`（已在 style.css 定义，列布局 + 12px gap）。
- 卡片：`section.panel-card` + `header.panel-card__header` + `div.panel-card__body`。
- 列表页统一结构：
  - 顶部 `metric-grid`（可选，4 列 KPI）
  - `panel-card > panel-card__header` 里放 `table-toolbar`：左侧 `table-toolbar__filters`（筛选控件 + 查询/重置），右侧 `table-toolbar__actions`（主操作按钮）
  - `panel-card__body` 里放 `n-data-table` + 分页
- 不要把 "新建 xxx" 按钮放在独立的顶部工具栏——全部融合到表格头工具栏右侧。
- 路由标题由 `DefaultLayout.vue` 的 `app-header` 统一渲染（`routeMetaMap`），页面内不再重复展示标题/副标题。因此 `AppPageHeader` 组件只承担"需要顶部操作条"时的容器角色。

### 侧边栏（DefaultLayout.vue）

- 折叠态宽度固定 64px，logo 区居中，菜单项居中。
- 折叠切换时：
  - 使用 `v-show` 而非 `v-if` 展示 brand meta，避免宽度抖动。
  - `app-brand--collapsed` 类统一 padding/gap，避免 logo 横向跳动。
  - 深度覆盖 `.n-menu-item-content--collapsed`：`justify-content: center`，图标 `position: static`，让图标真正居中。
- 不使用绝对定位的侧边栏页脚（会随内容变化抖动）。

### 弹窗 / 抽屉

- **新建/编辑类 Drawer 默认允许点击遮罩关闭**（`:mask-closable="true"`）。破坏性确认才用 `false`。
- 弹窗里避免使用 `n-alert` 堆叠说明文字，能放在按钮边、label 边或表单后 hint 即可。
- Token 管理等"可选多模型"的下拉选择：下拉框占满宽度，"不配置时默认不限制模型"这种提示放在下拉框 **下面**，用 `.form-hint` class。

### 表格列宽

- ID 70px，状态 86~90px，时间列 160~170px。
- 名称/主字段用 `minWidth`，其它列用定宽，保证视觉节奏一致。
- 操作列 fixed:'right'，宽度 140~170px，操作按钮用 `class="table-action-btn"` + `NTooltip` 圆形图标。

## 后端（backend/）

使用 Go + Gin（底层由标准 `net/http` Server 承载），结构：
- `api/` — 路由处理器
- `middleware/` — 鉴权等中间件
- `service/proxy/` — 代理核心
- `migration/` — 数据迁移
- `models/` — GORM 模型

## 本地开发

```bash
# 前端
cd frontend
npm install
npm run dev
npm run build   # 会先跑 vue-tsc 类型检查

# 后端
cd backend
go run ./cmd/...
```

## 修改 UI 时的自查清单

1. 有没有在 `n-form-item` 下放 `<p>` 或 `<n-alert>` 类的说明文字？如果有，改成 placeholder 或 `.form-hint`。
2. 表格页是否使用统一的 `table-toolbar` 结构？
3. 新建/编辑按钮是否放在表格头工具栏右侧？
4. 折叠菜单栏在切换时是否有宽度抖动？
5. 新建类弹窗/抽屉是否允许点击遮罩关闭？
