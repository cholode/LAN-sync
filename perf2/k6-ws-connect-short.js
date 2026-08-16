import ws from 'k6/ws'
import { check } from 'k6'

const HOST = __ENV.PERF_WS_HOST || 'ws://8.130.151.211'
const PATH = __ENV.PERF_WS_PATH || '/api/v1/ws'
const users = JSON.parse(open('users.json'))
const HOLD_SECONDS = Number(__ENV.PERF_HOLD_SECONDS || 5)

export default function () {
  const user = users[(__VU - 1) % users.length]
  const url = `${HOST}${PATH}?token=${encodeURIComponent(user.token)}`

  const res = ws.connect(url, {}, (socket) => {
    socket.on('open', () => {
      socket.setInterval(() => socket.ping(), 3000)
      socket.setTimeout(() => socket.close(), HOLD_SECONDS * 1000)
    })
    socket.on('message', () => {})
    socket.on('close', () => {})
    socket.on('error', () => {})
  })

  check(res, {
    'ws status 101': (r) => r && r.status === 101,
  })
}
