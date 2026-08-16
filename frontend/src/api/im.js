import { http } from './http.js'

function numeric(value) {
  const n = Number(value)
  return Number.isFinite(n) ? n : value
}

function normalizeRoom(room = {}) {
  return {
    ...room,
    id: room.id ?? numeric(room.room_id),
    name: room.name ?? room.room_name ?? `群聊 #${room.room_id ?? room.id ?? ''}`,
    last_message: room.last_message || room.last_msg || room.last_content || '',
    time: room.time || room.last_message_at || '',
  }
}

function normalizeMember(member = {}) {
  return {
    ...member,
    id: member.id ?? numeric(member.user_id),
    user_id: member.user_id ?? member.id,
    name: member.name ?? member.username ?? '',
    username: member.username ?? member.name ?? '',
    avatar: member.avatar || member.avatar_url || '',
  }
}

function normalizeMessage(message = {}) {
  return {
    ...message,
    id: message.id ?? message.msg_id ?? message.client_msg_id,
    user_id: message.user_id ?? numeric(message.sender_id),
    sender_id: message.sender_id ?? message.user_id,
    username: message.username || message.sender_name || message.user_name || '',
    avatar: message.avatar || message.sender_avatar || '',
    created_at: message.created_at || message.timestamp || '',
  }
}

function query(params = {}) {
  const sp = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') sp.set(key, String(value))
  })
  const text = sp.toString()
  return text ? `?${text}` : ''
}

function normalizeList(data) {
  return Array.isArray(data) ? data : (data?.items || data?.rooms || data?.members || data?.messages || data?.data || [])
}

export function normalizeSocketMessage(raw = {}) {
  return normalizeMessage({
    ...raw,
    id: raw.id ?? raw.ID ?? raw.client_msg_id ?? raw.ClientMsgID,
    room_id: raw.room_id ?? raw.RoomID ?? raw.roomID,
    sender_id: raw.sender_id ?? raw.SenderID ?? raw.senderID,
    client_msg_id: raw.client_msg_id ?? raw.ClientMsgID,
    content: raw.content ?? raw.Content ?? '',
    type: raw.type ?? raw.Type,
    created_at: raw.created_at ?? raw.CreatedAt,
  })
}

export const imApi = {
  async myRooms() {
    const data = await http.get('/my_rooms')
    return normalizeList(data).map(normalizeRoom)
  },
  async createRoom(payload) {
    const data = await http.post('/rooms', payload)
    const roomId = data?.room_id ?? data?.id
    if (roomId) {
      return normalizeRoom({
        ...data,
        id: roomId,
        name: data?.name ?? data?.room_name ?? payload?.name ?? payload?.room_name,
      })
    }
    return data
  },
  presignUpload: (payload) => http.post('/files/presign', payload),
  completeUpload: (payload) => http.post('/files/complete', payload),
  downloadUrl: (objectKey) => `${(import.meta.env.VITE_API_BASE || '/api/v1').replace(/\/$/, '')}/download/${encodeURIComponent(objectKey)}`,
  async messages(roomId) {
    const data = await http.get(`/rooms/${roomId}/messages`)
    return normalizeList(data).map(normalizeMessage)
  },
  async searchMessages(roomId, params = {}) {
    const data = await http.get(`/rooms/${roomId}/messages/search${query(params)}`)
    const hits = data?.hits?.hits || data?.hits || []
    if (Array.isArray(hits)) {
      return hits.map((item) => normalizeMessage(item._source ? { ...item._source, id: item._id } : item))
    }
    return normalizeList(data).map(normalizeMessage)
  },
  async members(roomId) {
    const data = await http.get(`/rooms/${roomId}/members`)
    return normalizeList(data).map(normalizeMember)
  },
  joinRoom: (roomId) => http.post(`/rooms/${roomId}/join`, {}),
  removeMember: (roomId, userId) => http.delete(`/rooms/${roomId}/members/${userId}`),
  disbandRoom: (roomId) => http.delete(`/rooms/${roomId}/disband`),
  enableAgent: (roomId) => http.post(`/rooms/${roomId}/agent/enable`, {}),
  disableAgent: (roomId) => http.post(`/rooms/${roomId}/agent/disable`, {}),
  removeAgent: (roomId) => http.delete(`/rooms/${roomId}/agent`),
  agentConfig: (roomId) => http.get(`/rooms/${roomId}/agent/config`),
  updateAgentConfig: (roomId, config) => http.put(`/rooms/${roomId}/agent/config`, config),
}

export function createImSocket(token) {
  const apiBase = import.meta.env.VITE_API_BASE || '/api/v1'
  const base = new URL(apiBase, window.location.origin)
  base.protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  base.pathname = `${base.pathname.replace(/\/$/, '')}/ws`
  const mode = import.meta.env.VITE_WS_AUTH_MODE || 'query'
  if (mode === 'query') {
    base.searchParams.set('token', token)
    return new WebSocket(base.toString())
  }
  return new WebSocket(base.toString(), ['lan-im', `jwt.${token}`])
}
