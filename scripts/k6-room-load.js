import ws from 'k6/ws';
import { sleep, check } from 'k6';
import { SharedArray } from 'k6/data';
import { Counter, Trend } from 'k6/metrics';

const users = new SharedArray('load-users', () => JSON.parse(open(__ENV.PERF_USERS_FILE)));
const rate = Number(__ENV.PERF_RATE_PER_SENDER);
const total = Number(__ENV.PERF_CONNECTIONS || 10000);
const membersPerRoom = Number(__ENV.PERF_MEMBERS_PER_ROOM || 10);
const sendersPerRoom = Number(__ENV.PERF_SENDERS_PER_ROOM || membersPerRoom);
const ramp = Number(__ENV.PERF_RAMP_SECONDS || 120);
const send = Number(__ENV.PERF_SEND_SECONDS || 300);
const quiet = Number(__ENV.PERF_QUIET_SECONDS || 60);
const receiveOnly = String(__ENV.PERF_RECEIVE_ONLY || 'false').toLowerCase() === 'true';
const heartbeatEnabled = String(__ENV.PERF_HEARTBEAT_ENABLED || 'true').toLowerCase() === 'true';
const runID = __ENV.PERF_RUN_ID;
if (!runID || !rate || !__ENV.PERF_WS_HOST || users.length < total) throw new Error('Explicit run ID, rate, host and user fixture are required');
if (sendersPerRoom > membersPerRoom) throw new Error('PERF_SENDERS_PER_ROOM cannot exceed PERF_MEMBERS_PER_ROOM');
const opened = new Counter('load_opened');
const sent = new Counter('load_sent');
const received = new Counter('load_received');
const receivedFrames = new Counter('load_received_frames');
const errors = new Counter('load_socket_errors');
const earlyClosed = new Counter('load_early_closed');
const closed = new Counter('load_closed');
const skipped = new Counter('load_send_slots_skipped');
const nonmonotonic = new Counter('load_nonmonotonic_delivery');
const malformed = new Counter('load_malformed_delivery');
const handshake = new Trend('load_handshake_ms', true);
const handshakeFailures = new Counter('load_handshake_failures');
const rampReconnects = new Counter('load_ramp_reconnects');
const rampConnectionErrors = new Counter('load_ramp_connection_errors');
const serverPings = new Counter('load_server_pings');
const serverPongs = new Counter('load_server_pongs');

export const options = {
  scenarios: { rooms: { executor: 'per-vu-iterations', vus: total, iterations: 1, maxDuration: `${ramp + send + quiet + 60}s` } },
  systemTags: ['status', 'scenario', 'error_code'],
  thresholds: { checks: ['rate==1'], load_socket_errors: ['count==0'], load_early_closed: ['count==0'] },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};
export function setup() {
  const t0 = Date.now() + 10000;
  console.log(`LOAD_T0=${t0}`);
  return { t0 };
}

export default function (data) {
  const index = __VU - 1;
  const user = users[index];
  const tSend = data.t0 + ramp * 1000;
  const tQuiet = tSend + send * 1000;
  const tClose = tQuiet + quiet * 1000;
  sleep(Math.max(0, (data.t0 + index * ramp * 1000 / total - Date.now()) / 1000));
  const prefix = `lt-${runID}-`;
  let sequence = 0, rx = 0, rxFrames = 0, tx = 0, skip = 0, nonmono = 0, bad = 0, pings = 0, pongs = 0;
  const highest = new Array(sendersPerRoom).fill(0);
  const flush = () => {
    if (rx) received.add(rx);
    if (rxFrames) receivedFrames.add(rxFrames);
    if (tx) sent.add(tx);
    if (skip) skipped.add(skip);
    if (nonmono) nonmonotonic.add(nonmono);
    if (bad) malformed.add(bad);
    if (pings) serverPings.add(pings);
    if (pongs) serverPongs.add(pongs);
    rx = rxFrames = tx = skip = nonmono = bad = pings = pongs = 0;
  };
  errors.add(0); earlyClosed.add(0); skipped.add(0); receivedFrames.add(0); handshakeFailures.add(0); rampReconnects.add(0); rampConnectionErrors.add(0); nonmonotonic.add(0); malformed.add(0);
  const connectOnce = () => {
    const started = Date.now();
    let didOpen = false;
    let openedAt = 0;
    let closedAt = 0;
    let sessionAlive = true;
    let response;
    response = ws.connect(`${__ENV.PERF_WS_HOST}/api/v1/ws?token=${encodeURIComponent(user.token)}`, {}, socket => {
      socket.on('open', () => {
        didOpen = true;
        openedAt = Date.now();
        if (heartbeatEnabled) socket.setInterval(() => socket.ping(), 10000);
        socket.setInterval(flush, 1000);
        socket.setTimeout(() => socket.close(), Math.max(1, tClose - Date.now()));
        if (index % membersPerRoom < sendersPerRoom) {
          const period = 1000 / rate;
          let next = tSend + ((Math.floor(index / membersPerRoom) * sendersPerRoom + index % membersPerRoom) / total) * period;
          const tick = () => {
            const now = Date.now();
            if (!sessionAlive || now >= tQuiet) return;
            if (now >= next) {
              const behind = Math.floor((now - next) / period);
              skip += behind;
              next += (behind + 1) * period;
              socket.send(JSON.stringify({ room_id: user.room_id, content: '#', client_msg_id: `${prefix}${index}-${++sequence}-${now}` }));
              tx++;
            }
            socket.setTimeout(tick, Math.max(1, Math.min(next, tQuiet) - Date.now()));
          };
          socket.setTimeout(tick, Math.max(1, next - Date.now()));
        }
      });
    socket.on('ping', () => { pings++; });
    socket.on('pong', () => { pongs++; });
    const receiveLine = raw => {
      let message;
      try { message = JSON.parse(raw); } catch { bad++; return; }
      const id = String(message.client_msg_id || message.ClientMsgID || '');
      if (!id.startsWith(prefix)) return;
      if (Number(message.room_id || message.RoomID) !== Number(user.room_id)) { bad++; return; }
      rx++;
      const fields = id.slice(prefix.length).split('-');
      const sender = Number(fields[0]) % membersPerRoom;
      const seq = Number(fields[1]);
      if (sender >= sendersPerRoom || !seq) { bad++; return; }
      if (seq <= highest[sender]) nonmono++;
      highest[sender] = Math.max(highest[sender], seq);
    };
    socket.on('message', raw => {
      if (receiveOnly) {
        rxFrames++;
        return;
      }
      for (const line of String(raw).split('\n')) {
        if (line.trim()) receiveLine(line);
      }
    });
    socket.on('error', () => {
      const now = Date.now();
      if (now < tSend) rampConnectionErrors.add(1);
      else if (now < tClose - 100) errors.add(1);
    });
    socket.on('close', () => {
      sessionAlive = false;
      closedAt = Date.now();
      flush();
    });
    });
    if (!closedAt) closedAt = Date.now();
    return { response, didOpen, openedAt, closedAt, handshakeMs: openedAt ? openedAt - started : 0 };
  };

  let attempt = 0;
  let session;
  while (Date.now() < tSend) {
    session = connectOnce();
    if (session.didOpen && session.closedAt >= tSend) break;
    rampReconnects.add(1);
    attempt++;
    const exponential = Math.min(5000, 100 * (2 ** Math.min(attempt - 1, 6)));
    const backoff = exponential * (0.8 + Math.random() * 0.4);
    sleep(Math.max(0, Math.min(backoff, tSend - Date.now())) / 1000);
  }

  flush();
  const connectedForLoad = session?.didOpen && session.openedAt <= tSend && session.closedAt >= tSend;
  if (!connectedForLoad) {
    handshakeFailures.add(1, { status: String(session?.response?.status || 'none') });
    check(session?.response, { 'connected by send phase': () => false });
    return;
  }

  opened.add(1);
  closed.add(1);
  handshake.add(session.handshakeMs);
  if (session.closedAt < tClose - 100) earlyClosed.add(1);
  check(session.response, {
    'connected by send phase': r => session.openedAt <= tSend && r && r.status === 101,
  });
}
