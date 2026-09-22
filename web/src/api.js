const base = ''

async function request(path, options = {}) {
  const timeoutMs = options.timeoutMs ?? 120000
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  const { timeoutMs: _timeout, ...fetchOptions } = options
  try {
    const res = await fetch(`${base}${path}`, {
      headers: { 'Content-Type': 'application/json', ...(fetchOptions.headers || {}) },
      signal: controller.signal,
      ...fetchOptions
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      const err = new Error(data.error || res.statusText)
      if (res.status === 401) err.code = 'LOCKED'
      throw err
    }
    return data
  } catch (e) {
    if (e.name === 'AbortError') {
      throw new Error('请求超时')
    }
    throw e
  } finally {
    clearTimeout(timer)
  }
}

export const api = {
  health: () => request('/api/health'),
  vaultInit: (password) =>
    request('/api/vault/init', {
      method: 'POST',
      body: JSON.stringify({ password })
    }),
  vaultUnlock: (password) =>
    request('/api/vault/unlock', {
      method: 'POST',
      body: JSON.stringify({ password })
    }),
  getSettings: () => request('/api/settings'),
  saveSettings: (body) =>
    request('/api/settings', { method: 'POST', body: JSON.stringify(body) }),
  cursorEnvStatus: () => request('/api/setup/cursor'),
  installCursorCLI: () =>
    request('/api/setup/cursor?action=install', {
      method: 'POST',
      timeoutMs: 300000
    }),
  testCursorConnection: () =>
    request('/api/setup/cursor?action=test', {
      method: 'POST',
      timeoutMs: 120000
    }),
  gitStatus: () => request('/api/git', { timeoutMs: 120000 }),
  gitInit: () =>
    request('/api/git?action=init', { method: 'POST', timeoutMs: 120000 }),
  gitSetRemote: (url) =>
    request('/api/git?action=set_remote', {
      method: 'POST',
      body: JSON.stringify({ url }),
      timeoutMs: 120000
    }),
  gitPushForce: () =>
    request('/api/git?action=push', { method: 'POST', timeoutMs: 300000 }),
  gitOverwrite: () =>
    request('/api/git?action=overwrite', { method: 'POST', timeoutMs: 300000 }),
  clipEnv: () => request('/api/clip/env'),
  clipText2SRT: (formData) =>
    request('/api/clip/text2srt', {
      method: 'POST',
      body: formData,
      headers: {},
      timeoutMs: 900000
    })
}
