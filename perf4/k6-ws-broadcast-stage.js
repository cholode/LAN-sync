import ws from 'k6/ws'
import { Counter, Trend } from 'k6/metrics'

const HOST = __ENV.PERF_WS_HOST || 'ws://8.130.151.211'
const PATH = __ENV.PERF_WS_PATH || '/api/v1/ws'
const users = JSON.parse(open('users.json'))

const roomId = Number(__ENV.PERF_ROOM_ID || 4)
const members = Number(__ENV.PERF_MEMBERS || 500)
const senders = Number(__ENV.PERF_SENDERS || 10)
const targetRate = Number(__ENV.PERF_MSG_RATE || 20)
const intervalMs = Math.max(1, 1000 / Math.max(1, targetRate / Math.max(1, senders)))
const holdMs = Number(__ENV.PERF_HOLD_SECONDS || 20) * 1000
const RAMP = Number(__ENV.PERF_RAMP_SECONDS || 30)
const HOLD_STAGE = Number(__ENV.PERF_STAGE_SECONDS || 20)

const wsConnected = new Counter('broadcast_ws_connected')
const sent = new Counter('broadcast_msg_sent')
const received = new Counter('broadcast_msg_received')
const e2e = new Trend('broadcast_e2e_ms')

export const options = {
  scenarios: {
    broadcast_ramp: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: `${RAMP}s`, target: members },
        { duration: `${HOLD_STAGE}s`, target: members },
      ],
      gracefulStop: '5s',
    },
  },
}

export default function () {
  const index = (__VU - 1) % members
  const user = users[index]
  const url = `${HOST}${PATH}?token=${encodeURIComponent(user.token)}`
  let seq = 0
  const isSender = index < senders

  ws.connect(url, {}, (socket) => {
    socket.on('open', () => {
      wsConnected.add(1)
      if (isSender) {
        socket.setInterval(() => {
          const cid = `p4-${Date.now()}-${index + 1}-${++seq}`
          socket.send(JSON.stringify({ room_id: roomId, content: 'x', client_msg_id: cid }))
          sent.add(1)
        }, intervalMs)
      }
      socket.setTimeout(() => socket.close(), holdMs)
    })

    socket.on('message', (data) => {
      let msg
      try { msg = JSON.parse(data) } catch { return }
      if (Number(msg.RoomID || msg.room_id) !== roomId) return
      received.add(1)

      const cid = msg.ClientMsgID || msg.client_msg_id || ''
      const parts = String(cid).split('-')
      if (parts.length === 4 && parts[0] === 'p4') {
        const sentAt = Number(parts[1])
        if (sentAt > 0) e2e.add(Date.now() - sentAt)
      }
    })

    socket.on('close', () => {})
    socket.on('error', () => {})
  })
}
