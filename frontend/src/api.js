const API = '/api/v1'

export const token = {
  get: () => localStorage.getItem('askflow_token') || '',
  set: value => localStorage.setItem('askflow_token', value),
  clear: () => localStorage.removeItem('askflow_token')
}

export async function request(path, options = {}) {
  const headers = { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...options.headers }
  if (token.get()) headers.Authorization = `Bearer ${token.get()}`
  const response = await fetch(`${API}${path}`, { ...options, headers })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || `请求失败 (${response.status})`)
  return data
}

export async function streamNotifications(onEvent, signal) {
  const response = await fetch(`${API}/notifications/stream`, { headers: { Authorization: `Bearer ${token.get()}` }, signal })
  if (!response.ok || !response.body) throw new Error('通知连接失败')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { value, done } = await reader.read()
    if (done) return
    buffer += decoder.decode(value, { stream: true })
    const blocks = buffer.split('\n\n')
    buffer = blocks.pop() || ''
    for (const block of blocks) {
      const line = block.split('\n').find(item => item.startsWith('data: '))
      if (line) onEvent(JSON.parse(line.slice(6)))
    }
  }
}
