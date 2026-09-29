// Resumable load-test fixture generator. Credentials stay under ignored data/.
// Run: PERF_API_BASE=http://host/api/v1 node scripts/seed-load-fixture.mjs [output-directory]
// Reuse the output directory to resume an interrupted run. Credentials stay under ignored data/.
import fs from 'node:fs';
import path from 'node:path';
import { randomBytes } from 'node:crypto';

const base = process.env.PERF_API_BASE || 'http://127.0.0.1/api/v1';
const total = Number(process.env.PERF_ACCOUNTS || 10000);
const roomCount = Number(process.env.PERF_ROOMS || 1000);
const membersPerRoom = Number(process.env.PERF_MEMBERS_PER_ROOM || 10);
const concurrency = Number(process.env.PERF_SEED_CONCURRENCY || 24);
if (total !== roomCount * membersPerRoom) throw new Error('PERF_ACCOUNTS must equal PERF_ROOMS * PERF_MEMBERS_PER_ROOM');
const out = path.resolve(process.argv[2] || `data/load-fixture-${Date.now()}`);
fs.mkdirSync(out, { recursive: true });
const stateFile = path.join(out, 'state.json');
const state = fs.existsSync(stateFile)
  ? JSON.parse(fs.readFileSync(stateFile, 'utf8'))
  : { prefix: `load_${Date.now().toString(36)}`, password: randomBytes(12).toString('hex'), users: [], rooms: [], created_at: new Date().toISOString() };
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
function save() {
  fs.writeFileSync(stateFile + '.tmp', JSON.stringify(state));
  fs.renameSync(stateFile + '.tmp', stateFile);
}
save();

async function request(route, { method = 'GET', token, body, allowConflict = false, attempts = 8 } = {}) {
  for (let attempt = 0; attempt < attempts; attempt++) {
    try {
      const response = await fetch(base + route, {
        method,
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: body === undefined ? undefined : JSON.stringify(body),
        signal: AbortSignal.timeout(20000),
      });
      if (response.ok) return await response.json();
      if (response.status === 409 && allowConflict) return {};
      if (![429, 502, 503, 504].includes(response.status) || attempt + 1 === attempts) {
        throw Object.assign(new Error(`${method} ${route}: HTTP ${response.status}`), { permanent: true });
      }
      await sleep(Math.max(1000, Number(response.headers.get('Retry-After') || 2) * 1000));
    } catch (error) {
      if (error.permanent || attempt + 1 === attempts) throw error;
      await sleep(Math.min(5000, 500 * (attempt + 1)));
    }
  }
}

async function pool(items, worker) {
  let next = 0;
  let failed = false;
  const results = await Promise.allSettled(Array.from({ length: concurrency }, async () => {
    while (!failed && next < items.length) {
      const item = items[next++];
      try { await worker(item); } catch (error) { failed = true; throw error; }
    }
  }));
  save();
  const failure = results.find(result => result.status === 'rejected');
  if (failure) throw failure.reason;
}

let phase = 'register/login';
const heartbeat = setInterval(() => {
  console.log(`${phase}: users=${state.users.filter(user => user?.token).length}/${total}, rooms=${state.rooms.filter(Boolean).length}/${roomCount}, assigned=${state.users.filter(user => user?.joined).length}/${total}`);
}, 20000);
console.log(`Output: ${out}`);
try {
  await pool(Array.from({ length: total }, (_, index) => index), async index => {
    if (state.users[index]?.token) return;
    const username = `${state.prefix}_${String(index + 1).padStart(5, '0')}`;
    const body = { username, password: state.password };
    await request('/register', { method: 'POST', body, allowConflict: true });
    const login = await request('/login', { method: 'POST', body });
    if (!login.token || !login.user?.id) throw new Error(`Missing login result for ${username}`);
    state.users[index] = { username, password: state.password, token: login.token, user_id: login.user.id, group_index: Math.floor(index / membersPerRoom) };
    if (index % 100 === 0) save();
  });

  phase = 'create rooms';
  await pool(Array.from({ length: roomCount }, (_, index) => index), async index => {
    if (state.rooms[index]) return;
    const owner = state.users[index * membersPerRoom];
    const name = `${state.prefix}_room_${String(index + 1).padStart(4, '0')}`;
    // Resolve an uncertain prior creation by name within this owner's rooms before retrying.
    let room;
    for (let attempt = 0; attempt < 4 && !room; attempt++) {
      const existing = await request('/my_rooms', { token: owner.token });
      room = existing.rooms.find(item => item.room_name === name);
      if (!room) {
        try { room = await request('/rooms', { method: 'POST', token: owner.token, body: { name }, attempts: 1 }); }
        catch (error) { if (attempt === 3) throw error; await sleep(1000); }
      }
    }
    if (!room?.room_id) throw new Error(`Could not create room ${name}`);
    state.rooms[index] = { room_id: room.room_id, name, creator_id: owner.user_id, group_index: index };
    owner.room_id = room.room_id;
    owner.joined = true;
    save();
  });

  phase = 'join rooms';
  await pool(state.users, async user => {
    if (user.joined) return;
    const room = state.rooms[user.group_index];
    await request(`/rooms/${room.room_id}/join`, { method: 'POST', token: user.token, body: {} });
    user.room_id = room.room_id;
    user.joined = true;
    if (user.user_id % 100 === 0) save();
  });

  phase = 'verify rooms';
  const verified = [];
  const allIDs = new Set();
  await pool(state.rooms, async room => {
    const expected = state.users.slice(room.group_index * membersPerRoom, (room.group_index + 1) * membersPerRoom);
    const actual = await request(`/rooms/${room.room_id}/members`, { token: expected[0].token });
    const ids = new Set(actual.members.map(member => member.user_id));
    if (actual.members.length !== membersPerRoom || ids.size !== membersPerRoom || expected.some(user => !ids.has(user.user_id))) {
      throw new Error(`Membership mismatch for room ${room.room_id}`);
    }
    for (const id of ids) {
      if (allIDs.has(id)) throw new Error(`User ${id} belongs to more than one fixture room`);
      allIDs.add(id);
    }
    verified.push({ ...room, member_count: ids.size, user_ids: expected.map(user => user.user_id) });
  });
  if (allIDs.size !== total || verified.length !== roomCount) throw new Error('Fixture total mismatch');
  const users = state.users.map(({ joined, group_index, ...user }) => user);
  verified.sort((a, b) => a.group_index - b.group_index);
  fs.writeFileSync(path.join(out, 'users.json'), JSON.stringify(users, null, 2));
  fs.writeFileSync(path.join(out, 'rooms.json'), JSON.stringify(verified, null, 2));
  const report = { base, accounts: users.length, rooms: verified.length, members_per_room: membersPerRoom, unique_members: allIDs.size, verified_at: new Date().toISOString(), prefix: state.prefix, room_ids: verified.map(room => room.room_id) };
  fs.writeFileSync(path.join(out, 'report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report));
} finally {
  clearInterval(heartbeat);
  save();
}
