<template>
  <el-row :gutter="16">
    <el-col :span="12">
      <el-card>
        <template #header>
          <div class="card-head">
            <span>Text → 字幕文件</span>
            <el-tag v-if="env" :type="env.ready ? 'success' : 'danger'" size="small">
              {{ env.message }}
            </el-tag>
          </div>
        </template>
        <el-form label-width="88px" @submit.prevent>
          <el-form-item label="音频文件">
            <el-upload
              :auto-upload="false"
              :limit="1"
              :on-change="onAudioChange"
              :on-remove="() => (audioFile = null)"
              accept=".mp3,.wav,.m4a,.mp4,audio/*,video/mp4"
              drag
              style="width: 100%"
            >
              <div class="upload-hint">
                拖入或点击选择 mp3 / wav / m4a / mp4
                <div class="sub">WhisperX 会把每句文本对齐到音频时间轴</div>
              </div>
            </el-upload>
          </el-form-item>
          <el-form-item label="字幕文本">
            <el-input
              v-model="text"
              type="textarea"
              :rows="10"
              placeholder="一行一句，例如：&#10;大家好，欢迎来到本期课程。&#10;今天我们讲三个要点。"
            />
            <div class="hint">一行一句。文案与口播一致才能对准；先改稿再对齐。</div>
          </el-form-item>
          <el-form-item label="语言">
            <el-radio-group v-model="language">
              <el-radio-button value="zh">中文</el-radio-button>
              <el-radio-button value="en">英文</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              :loading="running"
              :disabled="!audioFile || !text.trim() || !env?.ready"
              @click="run"
            >
              生成 SRT
            </el-button>
          </el-form-item>
        </el-form>
        <el-alert v-if="error" type="error" :title="error" show-icon :closable="false" />
      </el-card>
    </el-col>

    <el-col :span="12">
      <el-card>
        <template #header>
          <div class="card-head">
            <span>SRT 预览</span>
            <el-button v-if="srt" size="small" type="primary" plain @click="download">
              下载 SRT
            </el-button>
          </div>
        </template>
        <pre v-if="srt" class="mono">{{ srt }}</pre>
        <el-empty v-else description="生成后在这里预览" :image-size="80" />
        <pre v-if="log" class="mono log">{{ log }}</pre>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const env = ref(null)
const audioFile = ref(null)
const text = ref('')
const language = ref('zh')
const running = ref(false)
const srt = ref('')
const log = ref('')
const error = ref('')

onMounted(async () => {
  try {
    env.value = await api.clipEnv()
  } catch (e) {
    ElMessage.error(e.message)
  }
})

function onAudioChange(file) {
  audioFile.value = file.raw
}

async function run() {
  running.value = true
  error.value = ''
  srt.value = ''
  log.value = ''
  try {
    const fd = new FormData()
    fd.append('audio', audioFile.value)
    fd.append('text', text.value)
    fd.append('language', language.value)
    const res = await api.clipText2SRT(fd)
    log.value = res.output || ''
    if (res.ok) {
      srt.value = res.srt || ''
      ElMessage.success('字幕已生成')
    } else {
      error.value = res.error || '生成失败，请查看输出'
    }
  } catch (e) {
    error.value = e.message
  } finally {
    running.value = false
  }
}

function download() {
  const blob = new Blob([srt.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = audioFile.value
    ? audioFile.value.name.replace(/\.[^.]+$/, '') + '.srt'
    : 'subtitle.srt'
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
.upload-hint .sub {
  margin-top: 4px;
  font-size: 12px;
  color: #c0c4cc;
}
.hint {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
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
