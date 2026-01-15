<template>
  <NaiveProvider>
    <router-view/>
  </NaiveProvider>
</template>

<script setup lang="ts">
import NaiveProvider from './components/NaiveProvider.vue'
</script>

<style>
html,
body,
#app {
  width: 100%;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial,
  'Noto Sans', sans-serif, 'Apple Color Emoji', 'Segoe UI Emoji', 'Segoe UI Symbol',
  'Noto Color Emoji';
  font-size: 14px;
  line-height: 1.5;
  color: #333;
  background-color: #f5f7fa;
}

/* 强制移除任何可能的限制 */
#app {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
}


</style>
