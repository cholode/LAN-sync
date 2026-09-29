import fs from 'node:fs';
import path from 'node:path';
import { spawn, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { gzipSync } from 'node:zlib';

const exec = promisify(execFile);
const [outArg, k6, rateArg] = process.argv.slice(2);
if (!outArg || !k6 || !Number(rateArg)) throw new Error('Usage: node run-room-load.mjs OUTPUT K6_EXE RATE_PER_SENDER');
const out = path.resolve(outArg);
const rawDir = path.join(out, 'metrics');
fs.mkdirSync(rawDir, { recursive: true });
const runID = Date.now().toString(36);
const rampSeconds = Number(process.env.PERF_RAMP_SECONDS || 120);
const sendSeconds = Number(process.env.PERF_SEND_SECONDS || 300);
const quietSeconds = Number(process.env.PERF_QUIET_SECONDS || 60);
const gatewayContainers = (process.env.PERF_GATEWAY_CONTAINERS || '').split(',').map(value => value.trim()).filter(Boolean);
const meta = { run_id: runID, connections: 10000, rooms: 100, senders_per_room: 10, rate_per_sender: Number(rateArg), ramp_seconds: rampSeconds, send_seconds: sendSeconds, quiet_seconds: quietSeconds, gateway_containers: gatewayContainers, launched_at: Date.now() };
const saveMeta = () => fs.writeFileSync(path.join(out, 'run.json'), JSON.stringify(meta, null, 2));
saveMeta();
let finished = false;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const append = (file, value) => fs.appendFileSync(path.join(out, file), JSON.stringify(value) + '\n');
const failures = error => append('capture-errors.jsonl', { at: Date.now(), error: error.message });
async function fetchText(url) {
  const response = await fetch(url, { signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error(`HTTP ${response.status}: ${url}`);
  return response.text();
}
async function waitForGatewayProxy() {
  const deadline = Date.now() + 30000;
  let consecutiveReady = 0;
  let lastStatus = 'no response';
  while (Date.now() < deadline) {
    try {
      // This path is intentionally absent. A 404 proves Nginx reached the
      // current Gateway container without creating a WebSocket connection.
      const response = await fetch('http://127.0.0.1/api/__load_preflight__', {
        signal: AbortSignal.timeout(2000),
      });
      lastStatus = response.status;
      consecutiveReady = response.status < 500 ? consecutiveReady + 1 : 0;
      if (consecutiveReady >= 3) return;
    } catch (error) {
      lastStatus = error.message;
      consecutiveReady = 0;
    }
    await sleep(500);
  }
  throw new Error(`Nginx to Gateway preflight did not stabilize: ${lastStatus}`);
}
async function loop(interval, work) {
  while (!finished) {
    const began = Date.now();
    try { await work(began); } catch (error) { failures(error); }
    await sleep(Math.max(0, interval - (Date.now() - began)));
  }
}
await fetchText('http://127.0.0.1:6060/metrics');
await waitForGatewayProxy();
const child = spawn(k6, ['run', '--quiet', '--out', 'experimental-prometheus-rw', '--tag', `testid=${runID}`, '--summary-export', path.join(out, 'k6-summary.json'), path.resolve('scripts/k6-room-load.js')], {
  windowsHide: true,
  env: { ...process.env, PERF_USERS_FILE: path.resolve('data/load-fixture-10000x100/users.json'), PERF_RATE_PER_SENDER: String(rateArg), PERF_WS_HOST: 'ws://127.0.0.1', PERF_RUN_ID: runID, PERF_CONNECTIONS: '10000', PERF_RAMP_SECONDS: String(rampSeconds), PERF_SEND_SECONDS: String(sendSeconds), PERF_QUIET_SECONDS: String(quietSeconds), K6_PROMETHEUS_RW_SERVER_URL: 'http://127.0.0.1:9090/api/v1/write', K6_PROMETHEUS_RW_TREND_STATS: 'p(95),p(99),avg,max' },
});
meta.k6_pid = child.pid;
saveMeta();
for (const [stream, filename] of [[child.stdout, 'k6.out'], [child.stderr, 'k6.err']]) {
  stream.on('data', data => {
    fs.appendFileSync(path.join(out, filename), data);
    const match = data.toString().match(/LOAD_T0=(\d+)/);
    if (match) { meta.t0 = Number(match[1]); saveMeta(); console.log(`Timeline start ${new Date(meta.t0).toISOString()}`); }
  });
}
const exit = new Promise((resolve, reject) => { child.once('error', reject); child.once('close', (code, signal) => resolve({ code, signal })); });
const tasks = [
  ...(process.env.PERF_SEQUENCER_METRICS ? [loop(2000, async at => {
    const raw = await fetchText(process.env.PERF_SEQUENCER_METRICS);
    fs.writeFileSync(path.join(rawDir, `sequencer-${at}.prom.gz`), gzipSync(raw));
  })] : []),
  ...(process.env.PERF_INGRESS_TOPIC && process.env.PERF_OUTPUT_TOPIC ? [loop(15000, async at => {
    const { stdout } = await exec('docker', ['exec', 'kafka', '/opt/kafka/bin/kafka-consumer-groups.sh', '--bootstrap-server', 'kafka:9092', '--describe', '--group', `im_sequencer_${process.env.PERF_INGRESS_TOPIC}`, '--group', `im_relay_${process.env.PERF_OUTPUT_TOPIC}`], { windowsHide: true, timeout: 14000, maxBuffer: 1024 * 1024 });
    const { stdout: archived } = await exec('docker', ['exec', 'im-redis', 'redis-cli', 'GET', `im:kafka:offset:{${process.env.PERF_OUTPUT_TOPIC}}:0`], { windowsHide: true, timeout: 5000 });
    append('kafka-lag.jsonl', { at, text: stdout, archiver_last_offset: /^\d+$/.test(archived.trim()) ? Number(archived.trim()) : null });
  })] : []),
  loop(2000, async at => {
    if (!gatewayContainers.length) {
      const raw = await fetchText('http://127.0.0.1:6060/metrics');
      fs.writeFileSync(path.join(rawDir, `gateway-${at}.prom.gz`), gzipSync(raw));
      return;
    }
    await Promise.all(gatewayContainers.map(async container => {
      const { stdout } = await exec('docker', ['exec', container, 'wget', '-qO-', 'http://127.0.0.1:6060/metrics'], { windowsHide: true, timeout: 5000, maxBuffer: 4 * 1024 * 1024 });
      const safeName = container.replace(/[^a-zA-Z0-9_.-]/g, '_');
      fs.writeFileSync(path.join(rawDir, `gateway-${safeName}-${at}.prom.gz`), gzipSync(stdout));
    }));
  }),
  loop(5000, async at => {
    const query = `{job=~"lan-im-.*"} or {testid="${runID}"}`;
    const raw = await fetchText('http://127.0.0.1:9090/api/v1/query?query=' + encodeURIComponent(query));
    fs.writeFileSync(path.join(rawDir, `prometheus-${at}.json.gz`), gzipSync(raw));
  }),
  loop(10000, async at => {
    const { stdout } = await exec('docker', ['stats', '--no-stream', '--format', '{{json .}}'], { windowsHide: true, timeout: 9000, maxBuffer: 1024 * 1024 });
    append('containers.jsonl', { at, stats: stdout.trim().split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line)) });
  }),
  loop(10000, async at => {
    const command = `$p=Get-Process -Id ${child.pid} -ErrorAction SilentlyContinue; $w=Get-Process vmmemWSL -ErrorAction SilentlyContinue; $m=Get-CimInstance Win32_OperatingSystem; [pscustomobject]@{k6WorkingSet=$p.WorkingSet64;k6Private=$p.PrivateMemorySize64;k6CPU=$p.CPU;wslWorkingSet=$w.WorkingSet64;freePhysicalKiB=$m.FreePhysicalMemory}|ConvertTo-Json -Compress`;
    const { stdout } = await exec('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', command], { windowsHide: true, timeout: 9000 });
    const sample = JSON.parse(stdout);
    append('host-client.jsonl', { at, ...sample });
    if (!meta.exited_at && (sample.k6WorkingSet > 6 * 1024 ** 3 || sample.freePhysicalKiB < 500 * 1024)) {
      meta.aborted_reason = 'Host memory guard (client >6 GiB or Windows free <500 MiB)';
      saveMeta();
      child.kill();
    }
  }),
];
const progress = setInterval(async () => {
  try {
    const raw = JSON.parse(await fetchText('http://127.0.0.1:9090/api/v1/query?query=' + encodeURIComponent('sum(im_ws_connections_active)')));
    const active = raw.data.result[0]?.value[1] || '0';
    const elapsed = meta.t0 ? Math.round((Date.now() - meta.t0) / 1000) : -1;
    const phase = elapsed < 0 ? 'initializing' : elapsed < rampSeconds ? 'ramp' : elapsed < rampSeconds + sendSeconds ? 'sending' : elapsed < rampSeconds + sendSeconds + quietSeconds ? 'quiet' : 'disconnect';
    console.log(`phase=${phase} elapsed=${elapsed}s active=${active}`);
  } catch (error) { failures(error); }
}, 20000);
try {
  const result = await exit;
  Object.assign(meta, { exited_at: Date.now(), ...result });
  saveMeta();
  console.log('k6 exit ' + JSON.stringify(result));
  await sleep(15000);
} finally {
  finished = true;
  clearInterval(progress);
  await Promise.allSettled(tasks);
  meta.capture_finished_at = Date.now();
  saveMeta();
}
console.log(`Results: ${out}`);
