import http from 'k6/http'
import { check, sleep } from 'k6'

const BASE = __ENV.PERF_BASE || 'http://8.130.151.211'
const API = `${BASE}/api/v1`
const users = JSON.parse(open('users.json'))
const rooms = JSON.parse(open('rooms.json'))
const roomIds = Object.values(rooms).map(Number)
const readUsers = users.slice(0, Number(rooms['1000']) - 1)

function allowedRooms(userIndex) {
  const allowed = []
  if (userIndex < Number(rooms['10']) - 1) allowed.push(Number(rooms['10']))
  if (userIndex < Number(rooms['100']) - 1) allowed.push(Number(rooms['100']))
  if (userIndex < Number(rooms['500']) - 1) allowed.push(Number(rooms['500']))
  if (userIndex < Number(rooms['1000']) - 1) allowed.push(Number(rooms['1000']))
  return allowed
}

function randomReadUser() {
  const index = Math.floor(Math.random() * readUsers.length)
  return { user: readUsers[index], index, rooms: allowedRooms(index) }
}

function authHeaders(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }
}

export default function () {
  if (__ENV.PERF_HTTP_MODE === 'login') {
    const user = users[Math.floor(Math.random() * users.length)]
    const headers = authHeaders(user.token)
    const payload = JSON.stringify({ username: user.username, password: user.password })
    const res = http.post(`${API}/login`, payload, { headers })
    check(res, {
      'login status 200': (r) => r.status === 200,
      'login has token': (r) => !!r.json('token'),
    })
    return
  }

  const picked = randomReadUser()
  const user = picked.user
  const headers = authHeaders(user.token)
  const roll = Math.random()
  const roomId = picked.rooms[Math.floor(Math.random() * picked.rooms.length)]

  if (roll < 0.3) {
    const res = http.get(`${API}/my_rooms`, { headers })
    check(res, { 'my_rooms status 200': (r) => r.status === 200 })
  } else if (roll < 0.6) {
    const res = http.get(`${API}/rooms/${roomId}/messages?limit=50`, { headers })
    check(res, { 'history status 200': (r) => r.status === 200 })
  } else if (roll < 0.8) {
    const res = http.get(`${API}/rooms/${roomId}/members`, { headers })
    check(res, { 'members status 200': (r) => r.status === 200 })
  } else {
    const res = http.post(`${API}/rooms/${roomId}/join`, '{}', { headers })
    check(res, { 'join status 200': (r) => r.status === 200 })
  }

  sleep(0.05)
}
