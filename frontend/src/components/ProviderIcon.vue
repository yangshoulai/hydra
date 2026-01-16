<template>
  <div class="provider-icon">
    <img
      v-if="iconURL"
      :src="iconURL"
      :alt="alt"
      :style="{
        width: size + 'px',
        height: size + 'px',
        objectFit: 'contain'
      }"
    />
    <span v-else class="no-icon">无</span>
  </div>
</template>

<script setup lang="ts">
interface Props {
  lobeIcon?: string  // 保留接口兼容性，但不再使用
  iconURL?: string
  alt?: string
  size?: number
}

const props = withDefaults(defineProps<Props>(), {
  lobeIcon: undefined,
  iconURL: undefined,
  alt: 'Icon',
  size: 24
})
</script>

<script lang="ts">
export default {
  name: 'ProviderIcon'
}
</script>

<style scoped>
.provider-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.no-icon {
  color: #999;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
