import { http } from './http.js'

function query(params = {}) {
  const sp = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') sp.set(key, String(value))
  })
  const text = sp.toString()
  return text ? `?${text}` : ''
}

function asNumber(value, fallback = 0) {
  const n = Number(value)
  return Number.isFinite(n) ? n : fallback
}

function roleName(value) {
  const map = {
    0: 'user',
    1: 'super_admin',
    2: 'moderator',
    3: 'operator',
    user: 'user',
    super_admin: 'super_admin',
    moderator: 'moderator',
    operator: 'operator',
  }
  return map[String(value).toLowerCase()] || map[value] || 'user'
}

function userStatus(item = {}) {
  if (item.online) return 'online'
  if (Number(item.status) === 1) return 'banned'
  return 'offline'
}

function normalizeUser(item = {}) {
  return {
    ...item,
    role: roleName(item.role ?? item.role_name),
    raw_role: item.role,
    status: userStatus(item),
    raw_status: item.status,
    rooms: item.rooms ?? item.room_count ?? 0,
    messages: item.messages ?? item.message_count ?? 0,
    violations: item.violations ?? item.violation_count ?? 0,
    last_active: item.last_active ?? item.last_active_at ?? item.last_login_at ?? item.created_at,
  }
}

function normalizeRoom(item = {}) {
  return {
    ...item,
    owner: item.owner ?? item.owner_name ?? item.owner_id ?? '—',
    members: item.members ?? item.member_count ?? 0,
    online: item.online ?? item.online_member_count ?? 0,
    messages_today: item.messages_today ?? item.today_message_count ?? 0,
    agent: item.agent ?? item.agent_enabled ?? false,
    violations: item.violations ?? item.violation_count ?? 0,
    last_active: item.last_active ?? item.last_active_at ?? item.created_at,
  }
}

function normalizeModerationItem(item = {}) {
  const action = item.action || item.penalty_status || (item.model_result && item.model_result !== 'safe' ? item.model_result : 'RecordOnly')
  const result = item.penalty_status || item.review_status || item.model_result || 'recorded'
  return {
    ...item,
    time: item.time || item.created_at,
    user: item.user || item.username || `User #${item.user_id || '—'}`,
    room: item.room || item.room_name || `Room #${item.room_id || '—'}`,
    risk: item.risk || item.risk_level || 'low',
    summary: item.summary || item.model_reason || item.original_msg || '—',
    action,
    result,
  }
}

function allSettledValue(result, fallback = {}) {
  return result.status === 'fulfilled' ? result.value : fallback
}

function filterUsers(items, status) {
  if (!status) return items
  if (status === 'banned') return items.filter((item) => item.status === 'banned')
  if (status === 'online') return items.filter((item) => item.status === 'online')
  if (status === 'offline') return items.filter((item) => item.status === 'offline')
  return items
}

function filterRooms(items, sort) {
  const copy = [...items]
  if (sort === 'members') copy.sort((a, b) => Number(b.members) - Number(a.members))
  else if (sort === 'created') copy.sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0))
  else copy.sort((a, b) => Number(b.messages_today) - Number(a.messages_today))
  return copy
}

function filterModerationItems(items, q, risk) {
  let copy = [...items]
  if (risk) copy = copy.filter((item) => item.risk_level === risk || item.risk === risk)
  if (q) {
    const needle = String(q).toLowerCase()
    copy = copy.filter((item) => {
      return [item.username, item.room_name, item.category, item.id, item.user_id, item.room_id]
        .some((value) => String(value ?? '').toLowerCase().includes(needle))
    })
  }
  return copy
}

export const adminApi = {
  async users(params = {}) {
    const d = await http.get(`/admin/users${query({ page: 1, page_size: 100, keyword: params.q || params.keyword })}`)
    const items = (d?.items || d?.users || []).map(normalizeUser)
    const filtered = filterUsers(items, params.status)
    return { ...d, items: filtered, users: filtered, total: filtered.length, page: d?.page || 1, page_size: d?.page_size || 100 }
  },
  async user(id) {
    return normalizeUser(await http.get(`/admin/users/${id}`))
  },
  userAction: (id, action) => http.post(`/admin/users/${id}/action`, { action }),

  async rooms(params = {}) {
    const d = await http.get(`/admin/rooms${query({ page: 1, page_size: 100, keyword: params.q || params.keyword })}`)
    const items = (d?.items || d?.rooms || []).map(normalizeRoom)
    const sorted = filterRooms(items, params.sort)
    return { ...d, items: sorted, rooms: sorted, total: sorted.length, page: d?.page || 1, page_size: d?.page_size || 100 }
  },
  async room(id) {
    return normalizeRoom(await http.get(`/admin/rooms/${id}`))
  },
  roomAction: (id, action, targetUserId = 0) => http.post(`/admin/rooms/${id}/action`, {
    action,
    target_user_id: Number(targetUserId) || 0,
  }),
  deleteRoom: (id) => http.delete(`/admin/rooms/${id}`),

  async moderation(params = {}) {
    const [listRes, dashboardRes] = await Promise.allSettled([
      http.get(`/admin/moderation${query({ page: 1, page_size: 100, risk_level: params.risk, username: params.q })}`),
      http.get('/admin/dashboard/moderation'),
    ])
    const list = allSettledValue(listRes, { items: [] })
    const dashboard = allSettledValue(dashboardRes, {})
    if (listRes.status === 'rejected' && dashboardRes.status === 'rejected') throw listRes.reason

    const items = (list.items || []).map(normalizeModerationItem)
    const filtered = filterModerationItems(items, params.q, params.risk)
    const manualReviewed = Number(dashboard.manual_review_count || 0)
    const reviewed = Number(dashboard.today_reviewed || 0)
    const summary = {
      reviewed,
      violation_rate: Number(((dashboard.violation_rate ?? 0) * 100).toFixed(2)),
      auto_actions: Number(dashboard.auto_kick_count || 0) + Number(dashboard.auto_ban_count || 0),
      pending_review: Math.max(0, reviewed - manualReviewed),
    }
    return { ...list, items: filtered, total: filtered.length, summary }
  },
  reviewModeration: async (id, payload = {}) => {
    const decision = payload.decision === 'false_positive' ? 'false_positive' : 'confirmed'
    await http.post(`/admin/moderation/${encodeURIComponent(id)}/action`, { action: decision })
    return { event: { review_status: decision } }
  },

  async agentConfig() {
    const cfg = await http.get('/admin/agent-config')
    return {
      ...cfg,
      similarity_threshold: cfg.rag_similarity_threshold ?? cfg.similarity_threshold ?? 0.7,
    }
  },
  async saveAgentConfig(payload = {}) {
    const body = {
      ...payload,
      rag_similarity_threshold: payload.similarity_threshold ?? payload.rag_similarity_threshold ?? 0.7,
    }
    delete body.similarity_threshold
    const cfg = await http.put('/admin/agent-config', body)
    return {
      ...cfg,
      similarity_threshold: cfg.rag_similarity_threshold ?? cfg.similarity_threshold ?? 0.7,
    }
  },
  agentConfigHistory: (params = {}) => http.get(`/admin/agent-config/history${query({ page: 1, page_size: 20, ...params })}`),
  rollbackAgentConfig: () => http.post('/admin/agent-config/rollback', {}),
  ragQueries: (params = {}) => http.get(`/admin/rag/queries${query({ page: 1, page_size: 20, ...params })}`),

  files: (params = {}) => http.get(`/admin/files${query({ page: 1, page_size: 100, ...params, keyword: params.q || params.keyword })}`),
  file: (id) => http.get(`/admin/files/${id}`),
  fileDownload: (id) => http.get(`/admin/files/${id}/download`),
  deleteFile: (id) => http.delete(`/admin/files/${id}`),
  scanFiles: () => http.get('/admin/files/scan'),
  cleanupFiles: () => http.post('/admin/files/cleanup', {}),

  toolCalls: (params = {}) => http.get(`/admin/tool-calls${query({ page: 1, page_size: 100, ...params })}`),
  audits: (params = {}) => http.get(`/admin/audit-logs${query({ page: 1, page_size: params.page_size || 100, keyword: params.q, action: params.action })}`),
}
