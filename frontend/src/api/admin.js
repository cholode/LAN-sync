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

function normalizeRuntime(runtime = {}) {
  const ws = runtime.websocket || {}
  const go = runtime.golang || {}
  const api = runtime.api || {}
  return {
    ws_connections: ws.current_connections ?? 0,
    goroutines: go.goroutines ?? 0,
    api_p95_ms: api.p95_latency_ms ?? 0,
    heap_mb: go.heap_alloc ? Number((go.heap_alloc / 1024 / 1024).toFixed(1)) : 0,
    error_rate: api.error_rate ?? 0,
  }
}

function normalizeAgentRuntime(agent = {}) {
  return {
    calls_today: agent.calls_today ?? 0,
    calls: agent.calls_today ?? 0,
    success_rate: Number(((agent.success_rate ?? 0) * 100).toFixed(1)),
    p95_ms: agent.p95_response_ms ?? 0,
    tokens: agent.total_tokens ?? 0,
    tool_calls: agent.tool_calls ?? 0,
    rag_calls: agent.rag_calls ?? 0,
    qdrant_p95_ms: agent.qdrant_query_avg_ms ?? 0,
    current_requests: agent.current_requests ?? 0,
    failure_rate: agent.failure_rate ?? 0,
  }
}

function normalizeHealth(items = []) {
  return items.map((item) => ({
    name: item.name,
    status: item.status,
    latency_ms: item.latency_ms ?? 0,
    error: item.error || '',
  }))
}

function normalizeAlerts(items = []) {
  return items.map((item) => ({
    id: item.id,
    level: item.level || 'info',
    title: item.name || item.title || '系统告警',
    detail: item.message || item.detail || '',
    time: item.created_at || item.updated_at || '',
  }))
}

function trafficTrend(traffic = {}) {
  const hourly = Array.isArray(traffic.hourly) ? traffic.hourly : []
  return hourly.map((point) => ({
    time: point.time || '',
    messages: point.count ?? 0,
    agent: 0,
  }))
}

function messageTypes(traffic = {}) {
  const map = traffic.type_distribution || {}
  const privateGroup = traffic.private_group || {}
  return [
    { name: '群聊文本', value: Number(privateGroup.group ?? map.text ?? 0) },
    { name: '私聊文本', value: Number(privateGroup.private ?? 0) },
    { name: '文件消息', value: Number(map.file || 0) },
    { name: 'Agent 消息', value: Number(map.agent || 0) },
  ]
}

function allSettledValue(result, fallback = {}) {
  return result.status === 'fulfilled' ? result.value : fallback
}

async function fetchOverview() {
  const [overviewRes, runtimeRes, trafficRes, healthRes, alertsRes] = await Promise.allSettled([
    http.get('/admin/dashboard/overview'),
    http.get('/admin/dashboard/runtime'),
    http.get('/admin/dashboard/message-traffic'),
    http.get('/admin/health'),
    http.get('/admin/alerts?page=1&page_size=20'),
  ])

  const overview = allSettledValue(overviewRes)
  if (overviewRes.status === 'rejected') throw overviewRes.reason

  const runtime = normalizeRuntime(allSettledValue(runtimeRes))
  const traffic = allSettledValue(trafficRes)
  const healthPayload = allSettledValue(healthRes)
  const alertPayload = allSettledValue(alertsRes)
  const sections = overview.sections || {}
  const moderation = overview.moderation || {}
  const agent = normalizeAgentRuntime(overview.agent || {})

  return {
    users: {
      total: sections.users?.total ?? 0,
      online: sections.users?.online ?? 0,
      active_today: sections.users?.active_today ?? 0,
      new_today: sections.users?.new_today ?? 0,
    },
    rooms: {
      total: sections.rooms?.total ?? 0,
      active: sections.rooms?.total ?? 0,
      new_today: sections.rooms?.new_today ?? 0,
    },
    messages: {
      today: sections.messages?.today ?? 0,
      per_minute: runtime.ws_connections > 0 ? 0 : 0,
      private_today: sections.messages?.private_today ?? 0,
      group_today: sections.messages?.group_today ?? 0,
      file_today: sections.messages?.file_today ?? 0,
      agent_today: sections.messages?.agent_today ?? 0,
    },
    moderation: {
      violations: moderation.today_violations ?? 0,
      violation_rate: Number(((moderation.violation_rate ?? 0) * 100).toFixed(2)),
      reviewed: moderation.today_reviewed ?? 0,
      pending_reviews: moderation.pending_reviews ?? 0,
    },
    runtime,
    agent,
    rag: overview.rag || {},
    system: overview.system || [],
    message_trend: trafficTrend(traffic),
    message_types: messageTypes(traffic),
    health: normalizeHealth(healthPayload.items || overview.system || []),
    alerts: normalizeAlerts(alertPayload.items || []),
  }
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
  overview: fetchOverview,

  async users(params = {}) {
    const d = await http.get(`/admin/users${query({ page: 1, page_size: 100, keyword: params.q || params.keyword })}`)
    const items = (d?.items || d?.users || []).map(normalizeUser)
    const filtered = filterUsers(items, params.status)
    return { ...d, items: filtered, users: filtered, total: filtered.length, page: d?.page || 1, page_size: d?.page_size || 100 }
  },
  async user(id) {
    return normalizeUser(await http.get(`/admin/users/${id}`))
  },
  deleteUser: (id) => http.delete(`/admin/users/${id}`),

  async rooms(params = {}) {
    const d = await http.get(`/admin/rooms${query({ page: 1, page_size: 100, keyword: params.q || params.keyword })}`)
    const items = (d?.items || d?.rooms || []).map(normalizeRoom)
    const sorted = filterRooms(items, params.sort)
    return { ...d, items: sorted, rooms: sorted, total: sorted.length, page: d?.page || 1, page_size: d?.page_size || 100 }
  },
  async room(id) {
    return normalizeRoom(await http.get(`/admin/rooms/${id}`))
  },
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

  async agentOverview() {
    const [agentRes, ragRes] = await Promise.allSettled([
      http.get('/admin/dashboard/agent'),
      http.get('/admin/dashboard/rag'),
    ])
    const agent = normalizeAgentRuntime(allSettledValue(agentRes))
    const rag = allSettledValue(ragRes, {})
    return {
      ...agent,
      vector_count: rag.vector_count ?? 0,
      embedding_queue: rag.embedding_queue ?? 0,
      qdrant_p95_ms: rag.qdrant_query_avg_ms ?? agent.qdrant_p95_ms ?? 0,
      qdrant_online: rag.qdrant_online ?? false,
    }
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

  async systemOverview() {
    const [runtimeRes, healthRes] = await Promise.allSettled([
      http.get('/admin/dashboard/runtime'),
      http.get('/admin/health'),
    ])
    const runtime = normalizeRuntime(allSettledValue(runtimeRes))
    const healthPayload = allSettledValue(healthRes, { items: [] })
    return { runtime, health: normalizeHealth(healthPayload.items || []) }
  },
  audits: (params = {}) => http.get(`/admin/audit-logs${query({ page: 1, page_size: params.page_size || 100, keyword: params.q, action: params.action })}`),
}
