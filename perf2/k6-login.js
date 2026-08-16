import http from 'k6/http'
import { check, sleep } from 'k6'

const BASE = __ENV.PERF_BASE || 'http://8.130.151.211'
const API = `${BASE}/api/v1`
const users = JSON.parse(open('users.json'))
const sleepSec = Number(__ENV.PERF_SLEEP || 1)

export default function () {
  const user = users[(__VU - 1) % users.length]
  const payload = JSON.stringify({ username: user.username, password: user.password })
  const res = http.post(`${API}/login`, payload, { headers: { 'Content-Type': 'application/json' } })
  check(res, {
    'login status 200': (r) => r.status === 200,
  })
  sleep(sleepSec)
}
