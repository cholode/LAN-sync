import ws from 'k6/ws'
import { Counter, Trend } from 'k6/metrics'

const HOST = __ENV.PERF_WS_HOST || 'ws://8.130.151.211'
const PATH = __ENV.PERF_WS_PATH || '/api/v1/ws'
const users = JSON.parse(open('users.json'))
const rooms = JSON.parse(open('rooms.json'))

const roomId = Number(__ENV.PERF_ROOM_ID || rooms['10'])
const senders = Number(__ENV.PERF_SENDERS || 10)
const targetRate = Number(__ENV.PERF_MSG_RATE || 50)
const intervalMs = Math.max(1, 1000 / Math.max(1, targetRate / senders))
const holdMs = Number(__ENV.PERF_HOLD_SECONDS || 15) * 1000

const sent = new Counter('ws_msg_sent')
const received = new Counter('ws_msg_received')
const e2e = new Trend('ws_msg_e2e_ms')

export default function () {
  const vuIndex = (__VU - 1) % senders
  const user = users[vuIndex]
  const url = `${HOST}${PATH}?token=${encodeURIComponent(user.token)}`
  let seq = 0

  ws.connect(url, {}, (socket) => {
    socket.on('open', () => {
      socket.setInterval(() => {
        const cid = `t2-${Date.now()}-${vuIndex + 1}-${++seq}`
        socket.send(JSON.stringify({ room_id: roomId, content: 'x', client_msg_id: cid }))
        sent.add(1)
      }, intervalMs)

      socket.setTimeout(() => socket.close(), holdMs)
    })

    socket.on('message', (data) => {
      let msg
      try { msg = JSON.parse(data) } catch { return }
      if (Number(msg.RoomID || msg.room_id) !== roomId) return
      received.add(1)

      const cid = msg.ClientMsgID || msg.client_msg_id || ''
      const parts = String(cid).split('-')
      if (parts.length === 4 && parts[0] === 't2') {
        const sentAt = Number(parts[1])
        if (sentAt > 0) e2e.add(Date.now() - sentAt)
      }
    })

    socket.on('close', () => {})
    socket.on('error', () => {})
  })
}
