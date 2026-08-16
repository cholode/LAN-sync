import ws from 'k6/ws'
import { check } from 'k6'

const HOST = __ENV.PERF_WS_HOST || 'ws://8.130.151.211'
const PATH = __ENV.PERF_WS_PATH || '/api/v1/ws'
const users = JSON.parse(open('users.json'))
const TARGET = Number(__ENV.PERF_TARGET_VUS || 100)
const RAMP = Number(__ENV.PERF_RAMP_SECONDS || 30)
const HOLD_STAGE = Number(__ENV.PERF_STAGE_SECONDS || 20)
const HOLD_SECONDS = Number(__ENV.PERF_HOLD_SECONDS || 5)

export const options = {
  scenarios: {
    connect_ramp: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: `${RAMP}s`, target: TARGET },
        { duration: `${HOLD_STAGE}s`, target: TARGET },
      ],
      gracefulStop: '5s',
    },
  },
}

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
