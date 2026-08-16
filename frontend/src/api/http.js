const API_BASE = (import.meta.env.VITE_API_BASE || '/api/v1').replace(/\/$/, '')

export class ApiError extends Error {
  constructor(message, status, data) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.data = data
  }
}

function authHeaders() {
  const token = localStorage.getItem('lan_im_token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export async function apiRequest(path, options = {}) {
  const isForm = options.body instanceof FormData
  const headers = { ...authHeaders(), ...(options.headers || {}) }
  if (!isForm && options.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json'

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers })
  const contentType = res.headers.get('content-type') || ''
  let data = null
  if (contentType.includes('application/json')) data = await res.json().catch(() => null)
  else data = await res.text().catch(() => '')

  if (!res.ok) {
    const message = data?.error || data?.message || data?.msg || `请求失败 (${res.status})`
    throw new ApiError(message, res.status, data)
  }
  return data
}

export const http = {
  get: (path) => apiRequest(path),
  post: (path, body) => apiRequest(path, { method: 'POST', body: body instanceof FormData ? body : JSON.stringify(body || {}) }),
  put: (path, body) => apiRequest(path, { method: 'PUT', body: JSON.stringify(body || {}) }),
  patch: (path, body) => apiRequest(path, { method: 'PATCH', body: JSON.stringify(body || {}) }),
  delete: (path) => apiRequest(path, { method: 'DELETE' }),
}
