<template>
  <el-card>
    <template #header>工作台</template>
    <p class="intro">骨架已就绪。业务页以后再长。</p>
    <el-descriptions :column="1" border size="small" style="max-width: 480px">
      <el-descriptions-item label="服务">{{ health?.service || '—' }}</el-descriptions-item>
      <el-descriptions-item label="保险库">
        {{ vaultLabel }}
      </el-descriptions-item>
      <el-descriptions-item label="数据目录">{{ health?.dataDir || '—' }}</el-descriptions-item>
    </el-descriptions>
    <el-button style="margin-top: 16px" type="primary" @click="$router.push('/settings')">
      去设置 Cursor
    </el-button>
  </el-card>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'

const health = ref(null)
const vaultLabel = computed(() => {
  if (!health.value) return '—'
  if (!health.value.initialized) return '未初始化'
  return health.value.unlocked ? '已解锁' : '未解锁'
})

onMounted(async () => {
  try {
    health.value = await api.health()
  } catch {
    health.value = null
  }
})
</script>

<style scoped>
.intro {
  margin: 0 0 16px;
  color: #606266;
  line-height: 1.7;
}
</style>
