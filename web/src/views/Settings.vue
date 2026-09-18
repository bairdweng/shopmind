<template>
  <el-row :gutter="16">
    <el-col :span="14">
      <el-card>
        <template #header>Cursor</template>
        <el-form label-width="140px" @submit.prevent>
          <el-form-item label="API Key">
            <el-input
              v-model="form.cursor_api_key"
              type="password"
              show-password
              :placeholder="apiKeyPlaceholder"
              clearable
              @focus="onApiKeyFocus"
            />
            <div class="hint">
              在
              <a href="https://cursor.com/dashboard" target="_blank" rel="noopener">Cursor Dashboard</a>
              申请；写入保险库，不进 Git。留空保存则保留已配置 Key
            </div>
          </el-form-item>
          <el-form-item label="agent 路径（可选）">
            <el-input
              v-model="form.cursor_agent_bin"
              placeholder="默认自动查找 ~/.local/bin/agent"
              clearable
            />
          </el-form-item>
          <el-form-item>
            <el-space wrap>
              <el-button type="primary" @click="save" :loading="saving">保存设置</el-button>
              <el-button @click="clearApiKey" :disabled="!settings?.cursor_api_key_set">清除 Key</el-button>
            </el-space>
          </el-form-item>
        </el-form>
      </el-card>
    </el-col>

    <el-col :span="10">
      <el-card>
        <template #header>环境状态</template>
        <el-descriptions v-if="status" :column="1" border>
          <el-descriptions-item label="状态">
            <el-tag :type="status.ready ? 'success' : 'warning'">{{ status.message }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="Cursor CLI">
            {{ status.agent_installed ? '已安装' : '未安装' }}
          </el-descriptions-item>
          <el-descriptions-item v-if="status.agent_path" label="agent 路径">
            <span class="mono">{{ status.agent_path }}</span>
          </el-descriptions-item>
          <el-descriptions-item v-if="status.agent_version" label="版本">
            {{ status.agent_version }}
          </el-descriptions-item>
          <el-descriptions-item label="API Key">
            {{ status.api_key_configured ? '已配置' : '未配置' }}
          </el-descriptions-item>
        </el-descriptions>
        <el-alert v-else-if="loadError" type="error" :title="loadError" show-icon />
        <el-space wrap style="margin-top: 16px">
          <el-button @click="refreshStatus" :loading="checking">刷新状态</el-button>
          <el-button type="warning" @click="installCLI" :loading="installing">
            检测并安装 CLI
          </el-button>
          <el-button type="success" @click="testConnection" :loading="testing">
            测试连接
          </el-button>
        </el-space>
      </el-card>

      <el-card v-if="logOutput" style="margin-top: 16px">
        <template #header>操作输出</template>
        <pre class="mono">{{ logOutput }}</pre>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const settings = ref(null)
const status = ref(null)
const loadError = ref('')
const saving = ref(false)
const checking = ref(false)
const installing = ref(false)
const testing = ref(false)
const logOutput = ref('')
const apiKeyTouched = ref(false)

const form = ref({
  cursor_api_key: '',
  cursor_agent_bin: ''
})

const apiKeyPlaceholder = computed(() => {
  if (settings.value?.cursor_api_key_set) {
    return `已配置 ${settings.value.cursor_api_key_hint || ''}，留空不修改`
  }
  return '粘贴 Cursor API Key'
})

function onApiKeyFocus() {
  apiKeyTouched.value = true
}

async function loadAll() {
  loadError.value = ''
  try {
    settings.value = await api.getSettings()
    status.value = await api.cursorEnvStatus()
    form.value.cursor_agent_bin = settings.value.cursor_agent_bin || ''
    form.value.cursor_api_key = ''
    apiKeyTouched.value = false
  } catch (e) {
    loadError.value = e.message
  }
}

onMounted(loadAll)

async function save() {
  saving.value = true
  try {
    const res = await api.saveSettings({
      cursor_agent_bin: form.value.cursor_agent_bin,
      update_api_key: apiKeyTouched.value && !!form.value.cursor_api_key,
      cursor_api_key: form.value.cursor_api_key
    })
    settings.value = res.settings
    form.value.cursor_api_key = ''
    apiKeyTouched.value = false
    status.value = await api.cursorEnvStatus()
    ElMessage.success('设置已保存')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function clearApiKey() {
  saving.value = true
  try {
    const res = await api.saveSettings({
      cursor_agent_bin: form.value.cursor_agent_bin,
      clear_api_key: true
    })
    settings.value = res.settings
    form.value.cursor_api_key = ''
    status.value = await api.cursorEnvStatus()
    ElMessage.success('已清除 API Key')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function refreshStatus() {
  checking.value = true
  try {
    status.value = await api.cursorEnvStatus()
    settings.value = await api.getSettings()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    checking.value = false
  }
}

async function installCLI() {
  installing.value = true
  logOutput.value = ''
  try {
    const res = await api.installCursorCLI()
    logOutput.value = res.output || ''
    status.value = res.status || (await api.cursorEnvStatus())
    if (res.ok) {
      ElMessage.success('Cursor CLI 安装完成')
    } else {
      ElMessage.warning(res.error || '安装未完成，请查看输出')
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    installing.value = false
  }
}

async function testConnection() {
  testing.value = true
  logOutput.value = ''
  try {
    const res = await api.testCursorConnection()
    logOutput.value = res.output || ''
    if (res.ok) {
      ElMessage.success('连接测试成功')
    } else {
      ElMessage.warning(res.error || '连接测试失败')
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    testing.value = false
  }
}
</script>

<style scoped>
.hint {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  word-break: break-all;
}
pre.mono {
  margin: 0;
  white-space: pre-wrap;
  background: #111;
  color: #ddd;
  padding: 12px;
  border-radius: 8px;
  max-height: 320px;
  overflow: auto;
}
</style>
