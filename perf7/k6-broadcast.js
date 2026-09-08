import ws from 'k6/ws'
import { SharedArray } from 'k6/data'
import { Counter, Trend, Rate } from 'k6/metrics'
const users = new SharedArray('users', () => JSON.parse(open(__ENV.PERF_USERS_FILE || './users.json')))
const host = __ENV.PERF_WS_HOST
if (!host) throw new Error('Set PERF_WS_HOST explicitly to the authorized test server')
const members = Number(__ENV.PERF_MEMBERS || 100)
const senders = Number(__ENV.PERF_SENDERS || 1)
const room = Number(__ENV.PERF_ROOM_ID)
const warmup = Number(__ENV.PERF_WARMUP_SECONDS || 30)
const duration = Number(__ENV.PERF_SEND_SECONDS || 60)
const rate = Number(__ENV.PERF_MSG_RATE || 10)
const testid = __ENV.PERF_TEST_ID || 'manual'
const topology = __ENV.PERF_TOPOLOGY || 'single'
if (!room || members > users.length || senders < 1 || senders > members || rate <= 0) throw new Error('Invalid room, users, senders or rate')
const connected = new Rate('connected_before_send')
const sent = new Counter('messages_sent')
const received = new Counter('unique_deliveries')
const duplicates = new Counter('duplicate_deliveries')
const errors = new Counter('socket_errors')
const latency = new Trend('delivery_latency_ms', true)
export const options = {
  scenarios: { broadcast: { executor: 'per-vu-iterations', vus: members, iterations: 1, maxDuration: (warmup + duration + 30) + 's' } },
  thresholds: { connected_before_send: ['rate==1'], duplicate_deliveries: ['count==0'], socket_errors: ['count==0'] },
  tags: { testid, topology, members: String(members) },
}
export function setup() { return { start: Date.now() + warmup * 1000, run: String(Date.now()) } }
export default function (data) {
  const index = __VU - 1
  const end = data.start + duration * 1000
  const seen = new Set()
  let seq = 0, judged = false
  duplicates.add(0); errors.add(0); received.add(0)
  ws.connect(host + '/api/v1/ws?token=' + encodeURIComponent(users[index].token), {}, socket => {
    socket.on('open', () => {
      judged = true
      connected.add(Date.now() < data.start)
      if (index < senders) socket.setInterval(() => {
        const now = Date.now()
        if (now < data.start || now >= end) return
        socket.send(JSON.stringify({room_id: room, content: 'gateway scale test', client_msg_id: 'scale-' + data.run + '-' + now + '-' + index + '-' + (++seq)}))
        sent.add(1)
      }, Math.max(1, 1000 * senders / rate))
      socket.setInterval(() => socket.ping(), 10000)
      socket.setTimeout(() => socket.close(), Math.max(1, end + 10000 - Date.now()))
    })
    socket.on('message', raw => {
      let msg
      try { msg = JSON.parse(raw) } catch { return }
      const id = String(msg.client_msg_id || msg.ClientMsgID || '')
      if (!id.startsWith('scale-' + data.run + '-') || Number(msg.room_id || msg.RoomID) !== room) return
      if (seen.has(id)) { duplicates.add(1); return }
      seen.add(id); received.add(1)
      latency.add(Date.now() - Number(id.split('-')[2]))
    })
    socket.on('error', () => errors.add(1))
    socket.on('close', () => { if (Date.now() < end) errors.add(1) })
  })
  if (!judged) connected.add(false)
}
