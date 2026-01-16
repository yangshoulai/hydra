# Hydra Dockerfile - 不同基础镜像对比

## 方案 1: Alpine（当前方案）- 最小镜像
FROM golang:1.25.6-alpine AS backend-builder

# ✅ 优点：镜像小（~5MB）
# ❌ 缺点：需要手动安装 gcc, musl-dev
RUN apk add --no-cache gcc musl-dev

---

## 方案 2: Debian - 标准镜像
FROM golang:1.25.6 AS backend-builder

# ✅ 优点：已预装 gcc，无需额外安装
# ❌ 缺点：镜像较大（~124MB）
# 无需 RUN apt-get install gcc

---

## 方案 3: Debian Slim - 折中方案
FROM golang:1.25.6-slim AS backend-builder

# ✅ 优点：镜像较小（~80MB），通常有 gcc
# ⚠️  可能需要：RUN apt-get update && apt-get install -y gcc libc-dev

---

## 方案 4: Distroless / Scratch - 最小生产镜像（多阶段构建）
FROM golang:1.25.6 AS backend-builder
# [构建步骤...]

FROM gcr.io/distroless/cc-debian12 AS production

# ✅ 优点：生产镜像极小（~20MB），只包含运行时
# ✅ 缺点：需要多阶段构建

---

## 最终生产镜像大小对比

| 基础镜像 | 构建时大小 | 最终生产镜像 | 说明 |
|---------|-----------|-------------|------|
| Alpine | ~5MB | ~8MB | 最小，但需手动安装 gcc |
| Debian | ~124MB | ~25MB | 标准，自带 gcc |
| Debian Slim | ~80MB | ~20MB | 折中 |
| Distroless | N/A | ~20MB | 最小，无 shell |

## 关键点

无论哪种方案，**都需要 gcc**，因为：
- ✅ 使用 CGO_ENABLED=1
- ✅ SQLite 驱动需要 C 编译器
- ✅ PostgreSQL 驱动可能也需要

**如果不想用 gcc**，唯一的方法是：
1. 改用纯 Go 的 SQLite 驱动：`modernc.org/sqlite`
2. 或者禁用 SQLite 支持
