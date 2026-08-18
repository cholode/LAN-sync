// perf6 本地种子：针对本地 127.0.0.1 注册+登录拿 token，输出 local_users.json
// 用法: node seed_local.mjs   （PERF_USERS / PERF_SEED_CONCURRENCY / PERF_BASE 可覆盖）
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const BASE = process.env.PERF_BASE || 'http://127.0.0.1:8080/api/v1'
const PASSWORD = process.env.PERF_PASSWORD || 'Perf@12345'
const N = Number(process.env.PERF_USERS || 2000)
const CONCURRENCY = Number(process.env.PERF_SEED_CONCURRENCY || 20)
const DIR = path.dirname(fileURLToPath(import.meta.url))
const OUT = path.join(DIR, 'local_users.json')

async function req(p, { method = 'GET', token, body } = {}) {
  const h = { 'Content-Type': 'application/json' }
  if (token) h.Authorization = `Bearer ${token}`
  const r = await fetch(`${BASE}${p}`, { method, headers: h, body: body ? JSON.stringify(body) : undefined })
  const t = await r.text()
  let d = null
  try { d = JSON.parse(t) } catch { d = t }
  return { status: r.status, data: d }
}

async function withRetry(fn, label, tries = 3) {
  for (let i = 1; i <= tries; i++) {
    try { return await fn() } catch (e) {
      if (i === tries) throw e
      await new Promise((r) => setTimeout(r, 400 * i))
    }
  }
}

async function regLogin(username) {
  const body = { username, password: PASSWORD }
  await withRetry(async () => {
    const reg = await req('/register', { method: 'POST', body })
    if (reg.status !== 200 && reg.status !== 409) throw new Error(`register ${username} -> ${reg.status}`)
  }, `register ${username}`)
  const lg = await withRetry(async () => {
    const r = await req('/login', { method: 'POST', body })
    if (r.status !== 200 || !r.data?.token) throw new Error(`login ${username} -> ${r.status}`)
    return r
  }, `login ${username}`)
  return { username, password: PASSWORD, token: lg.data.token, user_id: lg.data.user?.id ?? null }
}

async function pool(items, worker, limit) {
  const out = new Array(items.length)
  let i = 0
  async function run() { while (true) { const idx = i++; if (idx >= items.length) return; out[idx] = await worker(items[idx]) } }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, run))
  return out
}

const names = Array.from({ length: N }, (_, i) => `perf_u_${String(i + 1).padStart(4, '0')}`)
let done = 0
const batch = 100
const users = []
for (let s = 0; s < names.length; s += batch) {
  const chunk = names.slice(s, s + batch)
  const created = await pool(chunk, regLogin, CONCURRENCY)
  users.push(...created)
  done += chunk.length
  fs.writeFileSync(OUT, JSON.stringify(users, null, 2), 'utf8')
  console.log(`seeded ${done}/${N}`)
}
console.log(`seed complete: ${users.length} users -> ${OUT}`)
