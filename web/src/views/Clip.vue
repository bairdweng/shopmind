<template>
  <el-row :gutter="16">
    <el-col :span="5">
      <el-card class="list-card">
        <template #header>
          <div class="card-head">
            <span>工具</span>
            <el-button link size="small" :loading="loading" @click="load">刷新</el-button>
          </div>
        </template>
        <div
          v-for="t in tools"
          :key="t.id"
          class="tool-item"
          :class="{ active: t.id === activeId }"
          @click="select(t)"
        >
          <span class="dot" :class="t.ready ? 'ok' : 'bad'" />
          <div class="meta">
            <div class="name">{{ t.name }}</div>
            <div class="desc">{{ t.description }}</div>
          </div>
        </div>
        <el-empty v-if="!loading && !tools.length" description="暂无工具" :image-size="60" />
      </el-card>
    </el-col>

    <el-col :span="19">
      <!-- 子路由渲染工具；keep-alive 按 id 缓存，切换工具后再回来表单与结果仍在 -->
      <router-view v-slot="{ Component, route: r }">
        <keep-alive>
          <component :is="Component" :key="r.params.id" />
        </keep-alive>
      </router-view>
      <el-empty v-if="!activeId" description="选择左侧工具开始" :image-size="80" />
    </el-col>
  </el-row>
</template>

<script setup>
import { computed, onMounted, provide, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const route = useRoute()
const router = useRouter()

const tools = ref([])
const loading = ref(false)
const activeId = computed(() => route.params.id || '')

// 工具列表作为单一数据源提供给子路由里的 ToolShell，避免重复请求。
provide('clipTools', tools)

async function load() {
  loading.value = true
  try {
    const res = await api.clipTools()
    tools.value = res.tools || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function select(t) {
  router.push(`/clip/${t.id}`)
}

onMounted(load)
</script>

<style scoped>
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.list-card {
  min-height: 400px;
}
.tool-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}
.tool-item:hover {
  background: #f5f7fa;
}
.tool-item.active {
  background: #ecf5ff;
}
.dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 6px;
}
.dot.ok {
  background: #67c23a;
}
.dot.bad {
  background: #f56c6c;
}
.meta {
  min-width: 0;
}
.name {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.desc {
  margin-top: 2px;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
}
</style>
