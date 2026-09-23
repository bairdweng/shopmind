<template>
  <el-card v-if="tool">
    <template #header>
      <div class="card-head">
        <span>{{ tool.name }}</span>
        <el-tag :type="env.ready ? 'success' : 'danger'" size="small">
          {{ env.message }}
        </el-tag>
      </div>
    </template>

    <el-form label-width="88px" @submit.prevent>
      <el-form-item v-for="f in visibleFields" :key="f.key" :label="f.label">
        <el-upload
          v-if="f.type === 'file'"
          :auto-upload="false"
          :limit="1"
          :on-change="(file) => (inputs[f.key] = file.raw)"
          :on-remove="() => (inputs[f.key] = null)"
          :accept="f.accept"
          drag
          style="width: 100%"
        >
          <div class="upload-hint">{{ f.hint || '拖入或点击选择文件' }}</div>
        </el-upload>

        <template v-else>
          <el-input
            v-if="f.type === 'textarea'"
            v-model="inputs[f.key]"
            type="textarea"
            :rows="f.rows || 6"
            :placeholder="f.placeholder"
          />
          <el-input
            v-else-if="f.type === 'text'"
            v-model="inputs[f.key]"
            :placeholder="f.placeholder"
          />
          <el-input-number
            v-else-if="f.type === 'number'"
            v-model="inputs[f.key]"
            :min="f.min"
            :max="f.max"
            :step="f.step"
            style="width: 180px"
          />
          <el-radio-group v-else-if="f.type === 'radio'" v-model="inputs[f.key]">
            <el-radio-button v-for="o in f.options" :key="o.value" :value="o.value">
              {{ o.label }}
            </el-radio-button>
          </el-radio-group>
          <div v-if="f.hint" class="hint">{{ f.hint }}</div>
        </template>
      </el-form-item>

      <el-form-item>
        <el-button type="primary" :loading="running" :disabled="!canRun" @click="run">
          {{ schema.submitText || '执行' }}
        </el-button>
      </el-form-item>
    </el-form>

    <el-alert v-if="error" type="error" :title="error" show-icon :closable="false" />

    <template v-if="resultText">
      <el-divider />
      <div class="out-head">
        <span>结果预览</span>
        <el-button size="small" type="primary" plain @click="download">
          下载 {{ filename }}
        </el-button>
      </div>
      <pre class="mono">{{ resultText }}</pre>
    </template>
    <pre v-if="log" class="mono log">{{ log }}</pre>
  </el-card>

  <el-alert
    v-else
    type="warning"
    title="未知工具或工具尚未加载，请刷新后重试"
    show-icon
    :closable="false"
  />
</template>

<script setup>
import { computed, inject, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { SCHEMAS } from '../views/clipSchemas'

const route = useRoute()
const id = route.params.id

// 工具信息由父级 Clip 提供（单一数据源，含环境就绪状态），不重复请求。
const tools = inject('clipTools', ref([]))
const tool = computed(() => tools.value.find((t) => t.id === id))
const env = computed(() => ({
  ready: tool.value?.ready ?? false,
  message: tool.value?.env_message || '检查环境中…'
}))

// 每个 id 对应独立的缓存实例，setup 只跑一次，schema 与输入按工具隔离。
const schema = SCHEMAS[id] || { submitText: '执行', fields: [] }

const inputs = reactive({})
for (const f of schema.fields) {
  inputs[f.key] = f.default ?? (f.type === 'file' ? null : '')
}

// 条件显示字段：schema 里用 show(inputs) 声明，依赖其它字段值动态出现。
const visibleFields = computed(() => schema.fields.filter((f) => !f.show || f.show(inputs)))

const running = ref(false)
const resultText = ref('')
const filename = ref('')
const log = ref('')
const error = ref('')

const canRun = computed(() => {
  if (!env.value.ready) return false
  return visibleFields.value.every((f) => !f.required || !!inputs[f.key])
})

async function run() {
  running.value = true
  error.value = ''
  resultText.value = ''
  filename.value = ''
  log.value = ''
  try {
    const fd = new FormData()
    for (const f of visibleFields.value) {
      const v = inputs[f.key]
      if (v !== null && v !== undefined && v !== '') fd.append(f.key, v)
    }
    const res = await api.clipRun(id, fd)
    log.value = res.output || ''
    if (res.ok) {
      resultText.value = res.text || ''
      filename.value = res.filename || 'output.txt'
      ElMessage.success('执行完成')
    } else {
      error.value = res.error || '执行失败，请查看输出'
    }
  } catch (e) {
    error.value = e.message
  } finally {
    running.value = false
  }
}

function download() {
  const blob = new Blob([resultText.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename.value
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.upload-hint {
  padding: 18px 0;
  color: #909399;
  font-size: 13px;
}
.hint {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
  width: 100%;
}
.out-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  margin: 0;
  white-space: pre-wrap;
  background: #111;
  color: #ddd;
  padding: 12px;
  border-radius: 8px;
  max-height: 420px;
  overflow: auto;
}
.mono.log {
  margin-top: 12px;
  max-height: 160px;
  color: #9ad;
}
</style>
