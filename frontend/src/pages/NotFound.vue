<template>
  <div class="not-found">
    <div class="not-found__code">404</div>
    <div class="not-found__title">页面不存在</div>
    <div class="not-found__desc">请求的路径 <code>{{ path }}</code> 没有对应的页面。</div>
    <n-space>
      <n-button @click="router.back()">返回上一页</n-button>
      <n-button type="primary" @click="router.push({ name: 'Dashboard' })">回到仪表盘</n-button>
    </n-space>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NSpace } from 'naive-ui'

const route = useRoute()
const router = useRouter()
const path = computed(() => route.fullPath)
</script>

<style scoped>
.not-found {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px 16px;
  color: var(--hydra-text);
}

.not-found__code {
  font-size: 72px;
  font-weight: 720;
  letter-spacing: 2px;
  line-height: 1;
  color: #111111;
}

.not-found__title {
  font-size: 18px;
  font-weight: 650;
  margin-top: 8px;
}

.not-found__desc {
  font-size: 13px;
  color: var(--hydra-text-tertiary);
  margin-bottom: 8px;
}

.not-found__desc code {
  padding: 2px 6px;
  border-radius: 4px;
  background: #f0f0f0;
  font-family: "JetBrains Mono", "SFMono-Regular", Menlo, Consolas, monospace;
  font-size: 12px;
}
</style>
