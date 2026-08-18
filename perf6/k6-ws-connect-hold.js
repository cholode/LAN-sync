// perf6: 只建连、不发消息的 WebSocket 连接容量压测
// 每个 VU 建立 1 条连接后保持（仅 ping 保活），直到场景结束
// 成功判定：socket 'open'（握手 101）；失败判定：socket 'error'（含非 101 响应）
// 注意：check 必须在事件回调里执行 —— 连接保持到场景结束会被 k6 gracefulStop 中断，
//       ws.connect 返回后的代码不会执行
// 用法: k6 run --summary-export summary_<level>.json k6-ws-connect-hold.js
import ws from 'k6/ws'
import { check } from 'k6'
import { open } from 'k6/experimental/fs'

const HOST = __ENV.PERF_WS_HOST || 'ws://8.130.151.211'
const PATH = __ENV.PERF_WS_PATH || '/api/v1/ws'

const TARGET = Number(__ENV.PERF_TARGET_VUS || 100)
const RAMP = Number(__ENV.PERF_RAMP_SECONDS || 30)
const HOLD = Number(__ENV.PERF_HOLD_SECONDS || 30)

// init 阶段只建句柄（不读内容），避免每个 VU 重复解析大 JSON（高 VU 下内存爆炸）
const file = await open(__ENV.PERF_USERS_FILE || 'users.json')

// 大数据集在 setup() 中只读取+解析一次，VU 共享同一份数据
// 说明：k6 无 TextDecoder，文件内容为纯 ASCII（用户名/密码/JWT），用分块 fromCharCode 解码
export async function setup() {
  const stat = await file.stat()
  const buf = new Uint8Array(Number(stat.size))
  const res = await file.read(buf)
  const bytes = res && res.bytesRead ? buf.slice(0, res.bytesRead) : buf
  let text = ''
  for (let i = 0; i < bytes.length; i += 8192) {
    text += String.fromCharCode.apply(null, bytes.subarray(i, i + 8192))
  }
  return JSON.parse(text)
}

export const options = {
  scenarios: {
    connect_hold: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: `${RAMP}s`, target: TARGET },
        { duration: `${HOLD}s`, target: TARGET },
      ],
      gracefulStop: '10s',
    },
  },
  thresholds: {
    checks: ['rate>0.95'],
  },
}

export default function (users) {
  const user = users[(__VU - 1) % users.length]
  const url = `${HOST}${PATH}?token=${encodeURIComponent(user.token)}`

  let judged = false
  function judge(ok) {
    if (judged) return
    judged = true
    check(ok, { 'ws status 101': (v) => v === true })
  }

  const res = ws.connect(url, {}, (socket) => {
    socket.on('open', () => {
      judge(true)
      // 保持连接：只发心跳，不发业务消息
      socket.setInterval(() => socket.ping(), 5000)
    })
    socket.on('error', () => {
      judge(false)
    })
    socket.on('close', () => {})
    socket.on('message', () => {})
  })

  // 兜底：握手失败（如非 101 响应）时 k6 不会进入回调，此时按返回值判定
  if (!judged) {
    judge(!!(res && res.status === 101))
  }
}
