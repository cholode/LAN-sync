import fs from 'node:fs';
import path from 'node:path';
import { spawn, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { gzipSync } from 'node:zlib';

const exec = promisify(execFile);
const [outArg, k6Arg, rateArg = '1'] = process.argv.slice(2);
if (!outArg) throw new Error('Usage: node scripts/run-cloud-room-load.mjs OUTPUT [K6_EXE] [RATE_PER_SENDER]');
const defaults = {
  k6: 'C:/Users/bocch/AppData/Local/Temp/lan-conn-memory-20260916-180056/bin/k6-v2.2.0-windows-amd64/k6.exe',
  workbench: `${process.env.LOCALAPPDATA}/Programs/workbench/workbench.exe`,
};
const k6 = path.resolve(k6Arg || process.env.PERF_K6_EXE || defaults.k6);
const workbench = path.resolve(process.env.PERF_WORKBENCH_EXE || defaults.workbench);
const instance = process.env.PERF_ECS_INSTANCE || 'i-f8zcl1p77evq5haq9svg';
const region = process.env.PERF_ECS_REGION || 'cn-heyuan';
const server = process.env.PERF_SERVER || '47.113.227.173';
const metricsMode = process.env.PERF_METRICS_MODE || 'workbench';
const grafanaUser = process.env.PERF_GRAFANA_USER || 'admin';
const grafanaPassword = process.env.PERF_GRAFANA_PASSWORD || '';
const grafanaDatasource = process.env.PERF_GRAFANA_DATASOURCE || 'lan-im-prometheus';
const clientMode = process.env.PERF_CLIENT_MODE || 'k6';
const fixture = path.resolve(process.env.PERF_FIXTURE_DIR || 'data/load-fixture-10000x1000');
const out = path.resolve(outArg);
const rawDir = path.join(out, 'metrics');
fs.mkdirSync(rawDir, { recursive: true });
if (!fs.existsSync(k6)) throw new Error(`k6 not found: ${k6}`);
if (!fs.existsSync(path.join(fixture, 'users.json'))) throw new Error(`Fixture not found: ${fixture}`);

const runID = Date.now().toString(36);
const connections = Number(process.env.PERF_CONNECTIONS || 10000);
const rooms = Number(process.env.PERF_ROOMS || 1000);
const membersPerRoom = Number(process.env.PERF_MEMBERS_PER_ROOM || 10);
const sendersPerRoom = Number(process.env.PERF_SENDERS_PER_ROOM || membersPerRoom);
const rampSeconds = Number(process.env.PERF_RAMP_SECONDS || 120);
const sendSeconds = Number(process.env.PERF_SEND_SECONDS || 180);
const quietSeconds = Number(process.env.PERF_QUIET_SECONDS || 0);
const meta = { run_id: runID, connections, rooms, members_per_room: membersPerRoom, senders_per_room: sendersPerRoom, rate_per_sender: Number(rateArg), ramp_seconds: rampSeconds, send_seconds: sendSeconds, quiet_seconds: quietSeconds, server, fixture, metrics_mode: metricsMode, client_mode: clientMode, launched_at: Date.now() };
const saveMeta = () => fs.writeFileSync(path.join(out, 'run.json'), JSON.stringify(meta, null, 2));
saveMeta();
let finished = false;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const append = (file, value) => fs.appendFileSync(path.join(out, file), JSON.stringify(value) + '\n');
const failure = error => append('capture-errors.jsonl', { at: Date.now(), error: error.message });

async function remoteDirect(command, timeout = 15000) {
  const { stdout } = await exec(workbench, ['exec', '-i', instance, '-r', region, '-c', command, '--timeout', String(Math.ceil(timeout / 1000)), '-o', 'json'], { windowsHide: true, timeout: timeout + 5000, maxBuffer: 16 * 1024 * 1024 });
  const result = JSON.parse(stdout.replace(/^\uFEFF/, ''));
  if (result.exit_code !== 0) throw new Error(result.stderr || `remote exit ${result.exit_code}`);
  return result.stdout;
}
// Workbench reuses one instance session. Serialize commands so sampling loops do
// not contend for that session and turn healthy remote commands into timeouts.
let remoteQueue = Promise.resolve();
function remote(command, timeout = 15000) {
  const task = remoteQueue.catch(() => {}).then(() => remoteDirect(command, timeout));
  remoteQueue = task;
  return task;
}
async function loop(interval, work) {
  while (!finished) {
    const began = Date.now();
    try { await work(began); } catch (error) { failure(error); }
    await sleep(Math.max(0, interval - (Date.now() - began)));
  }
}
async function captureGateways(at) {
  if (metricsMode === 'grafana') {
    if (!grafanaPassword) throw new Error('PERF_GRAFANA_PASSWORD is required in grafana metrics mode');
    const match = encodeURIComponent('{__name__=~"im_.*"}');
    const response = await fetch(`http://${server}:3000/api/datasources/proxy/uid/${grafanaDatasource}/federate?match%5B%5D=${match}`, {
      headers: { Authorization: `Basic ${Buffer.from(`${grafanaUser}:${grafanaPassword}`).toString('base64')}` },
      signal: AbortSignal.timeout(10000),
    });
    if (!response.ok) throw new Error(`Grafana Prometheus federation HTTP ${response.status}`);
    fs.writeFileSync(path.join(rawDir, `gateway-prometheus-${at}.prom.gz`), gzipSync(await response.text()));
    return;
  }
  const raw = await remote("for c in im-backend im-gateway-2 im-gateway-3; do printf '\\n__GATEWAY__%s\\n' \"$c\"; docker exec \"$c\" wget -qO- http://127.0.0.1:6060/metrics; done", 10000);
  for (const block of raw.split(/\n__GATEWAY__/).slice(1)) {
    const newline = block.indexOf('\n');
    const name = block.slice(0, newline).trim();
    fs.writeFileSync(path.join(rawDir, `gateway-${name}-${at}.prom.gz`), gzipSync(block.slice(newline + 1)));
  }
}

await fetch(`http://${server}/health/ready`, { signal: AbortSignal.timeout(5000) }).then(response => { if (!response.ok) throw new Error(`Server readiness HTTP ${response.status}`); });
await captureGateways(Date.now());
let command = k6;
let args = ['run', '--quiet', '--summary-export', path.join(out, 'k6-summary.json'), path.resolve('scripts/k6-room-load.js')];
if (clientMode === 'go') {
  const executable = path.join(out, 'ws-silent-load.exe');
  await exec('go', ['build', '-o', executable, path.resolve('scripts/ws-silent-load.go')], { windowsHide: true, timeout: 120000, maxBuffer: 4 * 1024 * 1024 });
  command = executable;
  args = ['-users', path.join(fixture, 'users.json'), '-host', `ws://${server}`, '-connections', String(connections), '-ramp', `${rampSeconds}s`, '-hold', `${sendSeconds + quietSeconds}s`, '-summary', path.join(out, 'k6-summary.json')];
}
let remoteSummary;
if (clientMode === 'remote-go') {
  remoteSummary = `/opt/lan-im/loadtest/summary-${runID}.json`;
  const launch = `docker rm -f im-load-client >/dev/null 2>&1 || true; rm -f ${remoteSummary}; docker run -d --name im-load-client --network host --memory 512m --memory-swap 512m --cpus 1.0 --ulimit nofile=262144:262144 -v /opt/lan-im/loadtest:/load alpine:3.23 /load/ws-silent-load -users /load/users.json -host ws://127.0.0.1 -connections ${connections} -ramp ${rampSeconds}s -hold ${sendSeconds + quietSeconds}s -summary /load/summary-${runID}.json`;
  await remote(launch, 30000);
  meta.t0 = Date.now() + 10000;
  command = 'powershell.exe';
  args = ['-NoProfile', '-NonInteractive', '-Command', `Start-Sleep -Seconds ${rampSeconds + sendSeconds + quietSeconds + 20}`];
}
const child = spawn(command, args, {
  windowsHide: true,
  env: {
    ...process.env,
    PERF_USERS_FILE: path.join(fixture, 'users.json'), PERF_RATE_PER_SENDER: String(rateArg), PERF_WS_HOST: `ws://${server}`, PERF_RUN_ID: runID,
    PERF_CONNECTIONS: String(connections), PERF_ROOMS: String(rooms), PERF_MEMBERS_PER_ROOM: String(membersPerRoom), PERF_SENDERS_PER_ROOM: String(sendersPerRoom),
    PERF_RAMP_SECONDS: String(rampSeconds), PERF_SEND_SECONDS: String(sendSeconds), PERF_QUIET_SECONDS: String(quietSeconds),
  },
});
meta.k6_pid = child.pid;
saveMeta();
for (const [stream, filename] of [[child.stdout, 'k6.out'], [child.stderr, 'k6.err']]) stream.on('data', data => {
  fs.appendFileSync(path.join(out, filename), data);
  const match = data.toString().match(/LOAD_T0=(\d+)/);
  if (match) { meta.t0 = Number(match[1]); saveMeta(); console.log(`Timeline start ${new Date(meta.t0).toISOString()}`); }
});
const exit = new Promise((resolve, reject) => { child.once('error', reject); child.once('close', (code, signal) => resolve({ code, signal })); });
const tasks = [
  loop(2000, captureGateways),
  loop(5000, async at => {
    const raw = metricsMode === 'grafana'
      ? JSON.stringify({ status: 'success', source: 'Grafana federation; see gateway snapshot at this timestamp' })
      : await remote("curl -fsSG http://127.0.0.1:9090/api/v1/query --data-urlencode 'query={__name__=~\"im_.*\"}'", 10000);
    fs.writeFileSync(path.join(rawDir, `prometheus-${at}.json.gz`), gzipSync(raw));
  }),
  loop(10000, async at => {
    if (metricsMode === 'grafana') { append('containers.jsonl', { at, stats: [], unavailable: 'Workbench control channel unavailable' }); return; }
    const raw = await remote("docker stats --no-stream --format '{{json .}}'", 12000);
    append('containers.jsonl', { at, stats: raw.trim().split(/\r?\n/).filter(Boolean).map(JSON.parse) });
  }),
  loop(10000, async at => {
    const command = `$p=Get-Process -Id ${child.pid} -ErrorAction SilentlyContinue; $m=Get-CimInstance Win32_OperatingSystem; [pscustomobject]@{k6WorkingSet=$p.WorkingSet64;k6Private=$p.PrivateMemorySize64;k6CPU=$p.CPU;wslWorkingSet=0;freePhysicalKiB=$m.FreePhysicalMemory}|ConvertTo-Json -Compress`;
    const { stdout } = await exec('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', command], { windowsHide: true, timeout: 9000 });
    const sample = JSON.parse(stdout);
    append('host-client.jsonl', { at, ...sample });
    if (!meta.exited_at && (sample.k6WorkingSet > 10 * 1024 ** 3 || sample.freePhysicalKiB < 750 * 1024)) {
      meta.aborted_reason = 'Client memory guard (k6 >10 GiB or Windows free <750 MiB)'; saveMeta(); child.kill();
    }
  }),
];
const progress = setInterval(async () => {
  try {
    let active;
    if (metricsMode === 'grafana') {
      const query = encodeURIComponent('sum(im_ws_connections_active)');
      const response = await fetch(`http://${server}:3000/api/datasources/proxy/uid/${grafanaDatasource}/api/v1/query?query=${query}`, { headers: { Authorization: `Basic ${Buffer.from(`${grafanaUser}:${grafanaPassword}`).toString('base64')}` }, signal: AbortSignal.timeout(10000) });
      active = (await response.json()).data?.result?.[0]?.value?.[1] || '0';
    } else {
      const raw = await remote("curl -fsSG http://127.0.0.1:9090/api/v1/query --data-urlencode 'query=sum(im_ws_connections_active)'", 10000);
      active = JSON.parse(raw).data.result[0]?.value[1] || '0';
    }
    const elapsed = meta.t0 ? Math.round((Date.now() - meta.t0) / 1000) : -1;
    const phase = elapsed < 0 ? 'initializing' : elapsed < rampSeconds ? 'ramp' : elapsed < rampSeconds + sendSeconds ? 'sending' : 'disconnect';
    console.log(`phase=${phase} elapsed=${elapsed}s active=${active}`);
  } catch (error) { failure(error); }
}, 20000);
try {
  const result = await exit;
  if (clientMode === 'remote-go') {
    const [summary, logs] = await Promise.all([remote(`cat ${remoteSummary}`, 15000), remote('docker logs im-load-client 2>&1', 15000)]);
    fs.writeFileSync(path.join(out, 'k6-summary.json'), summary);
    fs.writeFileSync(path.join(out, 'k6.out'), logs);
  }
  Object.assign(meta, { exited_at: Date.now(), ...result }); saveMeta(); console.log(`k6 exit ${JSON.stringify(result)}`); await sleep(10000);
} finally {
  finished = true; clearInterval(progress); await Promise.allSettled(tasks); await captureGateways(Date.now()).catch(failure);
  if (clientMode === 'remote-go') await remote('docker rm -f im-load-client >/dev/null 2>&1 || true', 15000).catch(failure);
  meta.capture_finished_at = Date.now(); saveMeta();
}
console.log(`Results: ${out}`);
