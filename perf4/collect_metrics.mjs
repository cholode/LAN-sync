import fs from 'node:fs'
import path from 'node:path'

const BASE = process.env.PERF_METRICS_URL || 'http://8.130.151.211:6060/metrics'
const prefix = process.argv[2] || 'sample'
const durationSec = Number(process.argv[3] || 60)
const intervalSec = Number(process.argv[4] || 2)
const outDir = path.join(process.cwd(), 'metrics')
fs.mkdirSync(outDir, { recursive: true })

let count = 0

async function collect() {
  try {
    const res = await fetch(BASE)
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const text = await res.text()
    const ts = Date.now()
    const file = path.join(outDir, `${prefix}_${ts}.txt`)
    fs.writeFileSync(file, text, 'utf8')
    count++
    console.log(`collector ${prefix} sample=${count} ts=${ts}`)
  } catch (err) {
    console.error(`collector ${prefix} error: ${err.message}`)
  }
}

collect()
const timer = setInterval(collect, intervalSec * 1000)
setTimeout(() => {
  clearInterval(timer)
  console.log(`collector ${prefix} done, samples=${count}`)
  process.exit(0)
}, durationSec * 1000 + 500)
