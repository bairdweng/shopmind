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

  <el-row :gutter="16" style="margin-top: 16px">
    <el-col :span="24">
      <el-card>
        <template #header>Git 数据同步（data/）</template>
        <el-alert
          v-if="git?.git_missing"
          type="error"
          title="未检测到 git 命令，请先安装 git"
          show-icon
          :closable="false"
        />
        <el-form label-width="140px" @submit.prevent>
          <el-form-item label="远端仓库地址">
            <el-input
              v-model="gitRemoteInput"
              :placeholder="git?.remote || '例如 git@github.com:you/shopmind-data.git'"
              clearable
            />
            <div class="hint">
              数据目录 data/（加密后的保险库）通过独立 Git 仓同步；master.key 等本机密钥在 .local/，永不进仓。
              在 GitHub 建一个私有空仓库，把地址填到上面。
            </div>
          </el-form-item>
          <el-form-item>
            <el-space wrap>
              <el-button
                type="primary"
                @click="saveRemote"
                :loading="gitBusy === 'remote'"
                :disabled="git?.git_missing"
              >
                保存远端并初始化
              </el-button>
              <el-button @click="loadGit" :loading="gitBusy === 'status'" :disabled="git?.git_missing">
                刷新状态
              </el-button>
              <el-button
                type="warning"
                @click="pushForce"
                :loading="gitBusy === 'push'"
                :disabled="git?.git_missing || !git?.is_repo || !git?.remote"
              >
                本地 → 远端（push -f 覆盖）
              </el-button>
              <el-button
                type="danger"
                @click="overwriteLocal"
                :loading="gitBusy === 'overwrite'"
                :disabled="git?.git_missing || !git?.is_repo || !git?.remote"
              >
                远端 → 本地（覆盖）
              </el-button>
            </el-space>
          </el-form-item>
        </el-form>

        <el-descriptions v-if="git" :column="2" border>
          <el-descriptions-item label="数据仓">
            <el-tag :type="git.is_repo ? 'success' : 'info'">
              {{ git.is_repo ? '已初始化' : '未初始化' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="分支">{{ git.branch || '—' }}</el-descriptions-item>
          <el-descriptions-item label="本地改动">
            {{ git.changed ? `${git.changed} 项未提交` : '干净' }}
          </el-descriptions-item>
          <el-descriptions-item label="与远端">
            <template v-if="git.ahead === null || git.ahead === undefined">待检查</template>
            <template v-else>
              领先 {{ git.ahead }} / 落后 {{ git.behind }}
            </template>
          </el-descriptions-item>
          <el-descriptions-item label="最新提交" :span="2">
            <span class="mono">{{ git.last_commit || '—' }}</span>
          </el-descriptions-item>
        </el-descriptions>

        <div class="hint" style="margin-top: 8px">
          覆盖是双向强制的：推送会以本地覆盖远端，拉取会丢弃本地未提交内容并重置为远端。
          拉取覆盖后保险库文件已更换，建议刷新页面重新解锁。
        </div>

        <pre v-if="gitLog" class="mono" style="margin-top: 12px">{{ gitLog }}</pre>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
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

const git = ref(null)
const gitRemoteInput = ref('')
const gitLog = ref('')
const gitBusy = ref('')

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
  loadGit()
}

async function loadGit() {
  gitBusy.value = 'status'
  try {
    git.value = await api.gitStatus()
    gitRemoteInput.value = git.value?.remote || ''
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    gitBusy.value = ''
  }
}

async function saveRemote() {
  const url = gitRemoteInput.value.trim()
  if (!url && git.value?.is_repo) {
    ElMessage.warning('请填写远端仓库地址')
    return
  }
  gitBusy.value = 'remote'
  gitLog.value = ''
  try {
    const res = url ? await api.gitSetRemote(url) : await api.gitInit()
    git.value = res.status
    gitLog.value = res.output || ''
    ElMessage.success(url ? '远端已保存' : '数据仓已初始化')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    gitBusy.value = ''
  }
}

async function pushForce() {
  try {
    await ElMessageBox.confirm(
      '将以本地数据强制覆盖远端仓库（push -f），远端上的新提交会被覆盖。继续？',
      '本地 → 远端',
      { type: 'warning', confirmButtonText: '强制推送', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  gitBusy.value = 'push'
  gitLog.value = ''
  try {
    const res = await api.gitPushForce()
    git.value = res.status
    gitLog.value = res.output || ''
    if (res.ok) {
      ElMessage.success('已推送到远端')
    } else {
      ElMessage.warning(res.error || '推送未完成，请查看输出')
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    gitBusy.value = ''
  }
}

async function overwriteLocal() {
  try {
    await ElMessageBox.confirm(
      '将丢弃本地未提交内容，用远端数据覆盖本地（reset --hard + clean）。覆盖后建议刷新页面重新解锁保险库。继续？',
      '远端 → 本地',
      { type: 'warning', confirmButtonText: '覆盖本地', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  gitBusy.value = 'overwrite'
  gitLog.value = ''
  try {
    const res = await api.gitOverwrite()
    git.value = res.status
    gitLog.value = res.output || ''
    if (res.ok) {
      ElMessage.success('已用远端覆盖本地')
    } else {
      ElMessage.warning(res.error || '覆盖未完成，请查看输出')
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    gitBusy.value = ''
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
