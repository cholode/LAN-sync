import assert from 'node:assert/strict'
import { setTimeout as delay } from 'node:timers/promises'

// 仅在一次性测试数据库和测试 Kafka 主题上运行，会创建测试账号、群和消息。
if (process.env.SMOKE_ISOLATED !== 'yes' || !process.env.SMOKE_BASE_URL) {
  throw new Error('请显式设置 SMOKE_ISOLATED=yes 和隔离环境的 SMOKE_BASE_URL')
}
const base = process.env.SMOKE_BASE_URL.replace(/\/$/, '')
const run = process.env.SMOKE_RUN_ID || String(Date.now())
const credentials = { username: `split_${run}`.slice(0, 32), password: 'split-smoke-password' }
let token = ''
async function request(path, method = 'GET', body, expected = 200) {
  const response = await fetch(base + '/api/v1' + path, {
    method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(10000),
  })
  assert.equal(response.status, expected, `${method} ${path} 状态码错误`)
  return response.json()
}

// 首轮注册，后续可复用同一测试标识检查 Worker 重启后的消息重放。
if (process.env.SMOKE_REUSE !== 'yes') await request('/register', 'POST', credentials)
token = (await request('/login', 'POST', credentials)).token
assert.ok(token)
let room
if (process.env.SMOKE_REUSE === 'yes') {
  room = (await request('/my_rooms')).rooms.find(item => item.room_name === `split-${run}`)
} else {
  room = await request('/rooms', 'POST', { name: `split-${run}` })
}
assert.ok(room?.room_id)
const roomID = room.room_id
assert.equal((await request(`/rooms/${roomID}/members`)).members.length, 1)
const contents = [`split-${run}-first`, `split-${run}-second`]
const observed = new Set()
const formalMessages = new Map()
const socket = new WebSocket(base.replace(/^http/, 'ws') + '/api/v1/ws?token=' + encodeURIComponent(token))
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error('WebSocket 握手超时')), 10000)
  socket.addEventListener('open', () => { clearTimeout(timer); resolve() }, { once: true })
  socket.addEventListener('error', () => { clearTimeout(timer); reject(new Error('WebSocket 连接失败')) }, { once: true })
})
socket.addEventListener('message', event => {
  const message = JSON.parse(String(event.data))
  const content = message.content || message.Content
  if (contents.includes(content)) {
    observed.add(content)
    const identity = { id: message.id ?? message.ID, seq: message.room_seq ?? message.RoomSeq }
    assert.equal(typeof identity.id, 'string', '实时消息 ID 必须保留字符串精度')
    assert.ok(BigInt(identity.seq) > 0n, '广播缺少群序号')
    if (formalMessages.has(content)) assert.deepEqual(identity, formalMessages.get(content), '重复广播改变了编号')
    formalMessages.set(content, identity)
  }
})
try {
  // 等待服务器完成房间订阅；发送同一凭证两次，检查主存储幂等。
  await delay(300)
  for (const index of [0, 0, 1]) socket.send(JSON.stringify({ room_id: roomID, content: contents[index], client_msg_id: `${run}-message-${index}` }))
  let history
  for (let attempt = 0; attempt < 30; attempt++) {
    history = await request(`/rooms/${roomID}/messages?limit=50`)
    history.messages = history.messages.filter(item => contents.includes(item.content))
    if (history.messages.length === 2 && observed.size === 2) break
    await delay(500)
  }
  assert.equal(observed.size, 2, '实时广播没有覆盖两条消息')
  assert.equal(history.messages.length, 2, '独立 Worker 未正确归档或重放产生重复记录')
  assert.deepEqual(new Set(history.messages.map(item => item.content)), new Set(contents))
  for (const item of history.messages) {
    assert.deepEqual({ id: item.id, seq: item.room_seq }, formalMessages.get(item.content), '广播和历史编号不一致')
    assert.ok(item.client_msg_id, '历史缺少客户端幂等凭证')
  }
  assert.ok(BigInt(history.messages[0].room_seq) < BigInt(history.messages[1].room_seq), '历史未按群序号排序')
  const search = await request(`/rooms/${roomID}/messages/search?q=${encodeURIComponent(contents[0])}`)
  assert.equal(search.total, 1, '消息搜索结果错误')
  const upload = await request('/files/presign', 'POST', { filename: 'split-smoke.txt', file_type: 'text/plain', file_size: 5 })
  assert.ok(upload.upload_url && upload.object_key, '独立文件 API 未返回上传凭证')
  await request('/files/complete', 'POST', {}, 400)
  token = ''
  await request(`/rooms/${roomID}/messages`, 'GET', undefined, 401)
  await request('/files/presign', 'POST', { filename: 'test.txt' }, 401)
  console.log(JSON.stringify({ result: '通过', checks: ['登录', '独立房间服务', 'WebSocket 广播', '独立归档', '重复消息幂等', '历史查询', '搜索', '文件凭证', 'JWT 鉴权'], messageIDs: history.messages.map(item => item.id) }))
} finally {
  socket.close()
}
