<template>
  <div v-if="bootError" class="gate">
    <el-alert type="error" :title="bootError" show-icon />
    <el-button style="margin-top: 12px" @click="loadHealth">重试</el-button>
  </div>
  <div v-else-if="!health" class="gate">加载中…</div>
  <div v-else-if="!health.unlocked" class="gate">
    <el-card class="gate-card">
      <template #header>{{ health.initialized ? '解锁保险库' : '初始化保险库' }}</template>
      <p class="gate-hint">
        {{
          health.initialized
            ? '输入公司/家里同一条口令。本机密钥写在 .local/keys，不进 Git。'
            : '首次使用：设一口令。主密钥只留本机，两台电脑用同一条口令各写一份。'
        }}
      </p>
      <el-form label-width="88px" @submit.prevent="submitGate">
        <el-form-item label="口令">
          <el-input v-model="password" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item v-if="!health.initialized" label="确认口令">
          <el-input v-model="password2" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="gating" @click="submitGate">
            {{ health.initialized ? '解锁' : '创建保险库' }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
  <el-container v-else class="layout">
    <el-aside class="aside" width="188px">
      <div class="brand" @click="$router.push('/')">ShopMind</div>
      <el-menu :default-active="activeMenu" class="aside-menu" router>
        <el-menu-item index="/clip">剪辑助手</el-menu-item>
        <el-menu-item index="/">工作台</el-menu-item>
        <el-menu-item index="/settings">设置</el-menu-item>
      </el-menu>
    </el-aside>
    <el-main class="main">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from './api'

const route = useRoute()
const health = ref(null)
const bootError = ref('')
const password = ref('')
const password2 = ref('')
const gating = ref(false)

const activeMenu = computed(() => {
  if (route.path.startsWith('/settings')) return '/settings'
  if (route.path.startsWith('/clip')) return '/clip'
  return '/'
})

async function loadHealth() {
  bootError.value = ''
  try {
    health.value = await api.health()
  } catch (e) {
    health.value = null
    bootError.value = e.message || '无法连接本地服务，请先 make serve-api'
  }
}

async function submitGate() {
  if (!password.value.trim()) {
    ElMessage.warning('请输入口令')
    return
  }
  if (!health.value.initialized && password.value !== password2.value) {
    ElMessage.warning('两次口令不一致')
    return
  }
  gating.value = true
  try {
    if (health.value.initialized) {
      await api.vaultUnlock(password.value)
    } else {
      await api.vaultInit(password.value)
    }
    password.value = ''
    password2.value = ''
    await loadHealth()
    ElMessage.success(health.value?.unlocked ? '已解锁' : '完成')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    gating.value = false
  }
}

onMounted(loadHealth)
</script>

<style scoped>
.layout {
  height: 100vh;
  overflow: hidden;
}
.aside {
  display: flex;
  flex-direction: column;
  background: #f7f7f5;
  border-right: 1px solid #e6e6e2;
  flex-shrink: 0;
}
.brand {
  min-height: 52px;
  display: flex;
  align-items: center;
  padding: 12px 16px;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: #1a1a18;
  cursor: pointer;
  user-select: none;
}
.aside-menu {
  border-right: none;
  background: transparent;
  flex: 1;
  padding: 4px 8px 16px;
}
.aside-menu :deep(.el-menu-item) {
  height: 40px;
  line-height: 40px;
  margin: 2px 0;
  border-radius: 8px;
  color: #5c5c56;
}
.aside-menu :deep(.el-menu-item:hover) {
  background: #ecece8;
  color: #1a1a18;
}
.aside-menu :deep(.el-menu-item.is-active) {
  background: #1a1a18;
  color: #f7f7f5;
  font-weight: 600;
}
.main {
  flex: 1;
  min-width: 0;
  overflow: auto;
  background: #ecece8;
  padding: 16px 20px;
}
.gate {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: #ecece8;
  padding: 24px;
}
.gate-card {
  width: 420px;
  max-width: 100%;
}
.gate-hint {
  margin: 0 0 16px;
  color: #606266;
  line-height: 1.6;
  font-size: 13px;
}
</style>

<style>
html,
body,
#app {
  height: 100%;
  margin: 0;
}
body {
  font-family: 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  background: #ecece8;
  color: #1a1a18;
}
</style>
