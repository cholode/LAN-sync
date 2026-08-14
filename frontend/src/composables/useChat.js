import { state } from '../store/index.js';
import { request, readErrorMessage } from '../api/api.js';
import {
  notifyLine,
  emitLiveMessage,
  onLiveMessage,
} from '../utils/systemMessage.js';

export { onLiveMessage };
import { loadAgentState } from './useAgent.js';

function cacheKeyFor(roomId) {
  return roomId == null ? 'global' : String(roomId);
}

function normalizeMessage(raw) {
  return {
    id: raw.id != null ? raw.id : raw.ID,
    sender_id: raw.sender_id != null ? raw.sender_id : raw.SenderID,
    content: raw.content != null ? raw.content : raw.Content,
    created_at: raw.created_at || raw.CreatedAt || new Date().toISOString(),
    kind: raw.kind || 'live',
  };
}

function getHistoryMeta(roomId) {
  const key = String(roomId);
  if (!state.historyMeta[key]) {
    state.historyMeta[key] = { cursor: 0, hasMore: false, loading: false };
  }
  return state.historyMeta[key];
}

function resetHistoryMeta(roomId) {
  state.historyMeta[String(roomId)] = {
    cursor: 0,
    hasMore: false,
    loading: false,
  };
}

export function connectWS() {
  if (state.ws) {
    state.ws.close();
  }

  state.ws = new WebSocket(state.wsBase + '?token=' + state.jwtToken);

  state.ws.onopen = () => notifyLine('实时连接已建立', 'sys');
  state.ws.onclose = () => notifyLine('连接已断开', 'err');
  state.ws.onerror = () => notifyLine('连接异常', 'err');

  state.ws.onmessage = (event) => {
    const frames = event.data.split('\n');
    frames.forEach((rawText) => {
      if (!rawText.trim()) return;

      try {
        const parsed = JSON.parse(rawText);
        const room = parsed.room_id != null ? parsed.room_id : parsed.RoomID;
        const content = parsed.content != null ? parsed.content : parsed.Content;
        const sender = parsed.sender_id != null ? parsed.sender_id : parsed.SenderID;
        const message = normalizeMessage({
          id: parsed.id,
          ID: parsed.ID,
          sender_id: sender,
          content,
          created_at: parsed.created_at || parsed.CreatedAt || new Date().toISOString(),
          kind: 'live',
        });

        if (message.id != null && state.loadedMessageIds.has(String(message.id))) {
          return;
        }
        if (message.id != null) {
          state.loadedMessageIds.add(String(message.id));
        }

        const key = cacheKeyFor(room);
        if (!state.messageCache[key]) {
          state.messageCache[key] = [];
        }
        state.messageCache[key].push(message);
        if (state.messageCache[key].length > state.MAX_CACHE_SIZE) {
          state.messageCache[key].shift();
        }

        const isCurrentRoom = room != null && String(room) === String(state.currentRoomId);
        if (isCurrentRoom || room === '全局') {
          state.messages.push(message);
          emitLiveMessage(message);
        }
      } catch (err) {
        notifyLine('收到无法解析的消息', 'err');
      }
    });
  };
}

export function initChat() {
  connectWS();
  loadMyRooms();
}

export function sendMessage(content) {
  if (!state.currentRoomId) return alert('请先选择一个群聊！');
  if (!state.ws || state.ws.readyState !== WebSocket.OPEN) return alert('连接未就绪！');

  const text = String(content || '').trim();
  if (!text) return;

  state.ws.send(
    JSON.stringify({
      room_id: state.currentRoomId,
      content: text,
      client_msg_id: crypto.randomUUID(),
    }),
  );
}

export async function loadMyRooms() {
  try {
    const res = await request('/my_rooms');
    if (!res.ok) return;
    const data = await res.json();
    state.rooms = data.rooms || data || [];
  } catch (e) {
    console.error('加载群聊列表失败', e);
  }
}

export async function selectRoom(roomId) {
  if (state.currentRoomId === roomId) return;

  resetHistoryMeta(roomId);

  if (state.ws && state.ws.readyState === WebSocket.OPEN) {
    try {
      await request('/rooms/' + roomId + '/join', { method: 'POST' });
    } catch (e) {
      // 加入失败不阻断本地切换。
    }
  }

  state.currentRoomId = roomId;
  const room = state.rooms.find((item) => (item.id || item.room_id) === roomId);
  state.currentRoomName = room ? room.name : '群聊 #' + roomId;
  state.messages = [];
  state.loadedMessageIds.clear();
  state.members = [];
  state.membersHint = '正在加载成员...';
  state.agent.visible = true;
  state.agent.enabled = false;

  const key = cacheKeyFor(roomId);
  const cached = state.messageCache[key] || [];
  if (cached.length > 0) {
    state.messages = cached.slice(-state.MAX_CACHE_SIZE);
  }

  loadRoomMembers(roomId);
  loadAgentState();
  await loadHistory(roomId, false);
  emitLiveMessage({ kind: 'history-loaded' });
}

export async function loadHistory(roomId, prepend = false) {
  const meta = getHistoryMeta(roomId);
  if (meta.loading) return false;
  meta.loading = true;

  const cursor = prepend ? meta.cursor : 0;
  const endpoint = '/rooms/' + roomId + '/messages?limit=50&cursor=' + cursor;

  try {
    state.historyHint = prepend ? '加载更多历史消息...' : '正在加载历史消息...';

    const res = await request(endpoint);
    if (!res.ok) {
      state.historyHint = '';
      if (!prepend && state.messages.length === 0) {
        state.messages = [
          {
            sender_id: null,
            content: '加载失败 (HTTP ' + res.status + ')，请重试',
            kind: 'err',
            created_at: new Date().toISOString(),
          },
        ];
      }
      return false;
    }

    const data = await res.json();
    const rawMessages = data.messages || data || [];
    const messages = rawMessages.map(normalizeMessage);
    const nextCursor = data.next_cursor ? Number(data.next_cursor) : 0;
    const hasMore = data.has_more === true;

    meta.cursor = nextCursor || 0;
    meta.hasMore = hasMore;

    if (messages.length === 0) {
      if (!prepend && state.messages.length === 0) {
        state.messages = [
          {
            sender_id: null,
            content: '暂无消息，发送第一条吧',
            kind: 'sys',
            created_at: new Date().toISOString(),
          },
        ];
      }
      state.historyHint = '没有更多历史消息';
      return false;
    }

    if (!prepend) {
      state.messages = messages;
      state.messageCache[cacheKeyFor(roomId)] = messages;
      state.loadedMessageIds = new Set(
        messages.filter((msg) => msg.id != null).map((msg) => String(msg.id)),
      );
    } else {
      const existing = new Set(
        state.messages
          .filter((msg) => msg.id != null)
          .map((msg) => String(msg.id)),
      );
      const added = messages.filter(
        (msg) => msg.id == null || !existing.has(String(msg.id)),
      );
      state.messages = added.concat(state.messages);
      state.messageCache[cacheKeyFor(roomId)] = state.messages.slice(
        -state.MAX_CACHE_SIZE,
      );
    }

    state.historyHint = hasMore ? '向上滚动加载更多历史消息' : '已加载全部历史消息';
    return true;
  } catch (e) {
    if (!prepend && state.messages.length === 0) {
      state.messages = [
        {
          sender_id: null,
          content: '加载失败：' + (e.message || '网络异常'),
          kind: 'err',
          created_at: new Date().toISOString(),
        },
      ];
    }
    state.historyHint = '';
    return false;
  } finally {
    meta.loading = false;
  }
}

export async function loadMoreHistory() {
  if (!state.currentRoomId) return false;
  const meta = getHistoryMeta(state.currentRoomId);
  if (!meta.hasMore || meta.loading) return false;
  return await loadHistory(state.currentRoomId, true);
}

export async function loadRoomMembers(roomId) {
  state.membersHint = '正在加载成员...';
  try {
    const res = await request('/rooms/' + roomId + '/members');
    if (!res.ok) {
      state.members = [];
      state.membersHint = '成员加载失败';
      return;
    }

    const data = await res.json();
    state.members = data.members || data || [];
    state.membersHint = '群聊 #' + roomId + ' · ' + state.members.length + ' 人';
  } catch (e) {
    state.members = [];
    state.membersHint = '成员加载失败';
  }
}

export async function createRoom(name) {
  const roomName = String(name || '').trim();
  if (!roomName) return alert('群聊名称不可为空');

  try {
    const res = await request('/rooms', {
      method: 'POST',
      body: JSON.stringify({ name: roomName }),
    });
    if (res.ok) {
      state.newRoomName = '';
      await loadMyRooms();
    } else {
      const err = await readErrorMessage(res);
      alert('创建群聊失败: ' + err);
    }
  } catch (e) {
    alert('网络异常');
  }
}

export async function joinRoom(roomId) {
  const id = Number(roomId);
  if (!id) return alert('请输入群聊ID');

  try {
    const res = await request('/rooms/' + id + '/join', { method: 'POST' });
    if (res.ok) {
      state.joinRoomId = '';
      await loadMyRooms();
    } else {
      const err = await readErrorMessage(res);
      alert('加入群聊失败: ' + err);
    }
  } catch (e) {
    alert('网络异常');
  }
}

export async function removeMember(targetId) {
  if (!state.currentRoomId) return alert('连接未就绪！');
  if (!targetId) return alert('请输入要移除的用户ID');

  const selfId = state.user ? Number(state.user.id || state.user.user_id) : null;
  const isSelfKick = selfId != null && Number(targetId) === selfId;

  if (isSelfKick && !confirm('确定退出当前群聊？')) return;
  if (!isSelfKick && !confirm('确定移除用户 #' + targetId + '？')) return;

  try {
    const res = await request(
      '/rooms/' + state.currentRoomId + '/members/' + targetId,
      { method: 'DELETE' },
    );
    if (res.ok) {
      state.kickUserId = '';
      if (isSelfKick) {
        state.currentRoomId = null;
        state.currentRoomName = '';
        state.messages = [];
        state.members = [];
        state.agent.visible = false;
      }
      await loadMyRooms();
      if (state.currentRoomId) {
        await loadRoomMembers(state.currentRoomId);
      }
    } else {
      const err = await readErrorMessage(res);
      alert('操作失败: ' + err);
    }
  } catch (e) {
    alert('网络异常');
  }
}

export async function disbandCurrentRoom() {
  if (!state.currentRoomId) return;
  if (!confirm('警告：解散群聊后不可恢复！确定解散 #' + state.currentRoomId + '？')) {
    return;
  }

  try {
    const res = await request('/rooms/' + state.currentRoomId + '/disband', {
      method: 'DELETE',
    });
    if (res.ok) {
      state.currentRoomId = null;
      state.currentRoomName = '';
      state.messages = [];
      state.members = [];
      state.membersHint = '暂无成员';
      state.agent.visible = false;
      await loadMyRooms();
    } else {
      const err = await readErrorMessage(res);
      alert('解散失败: ' + err);
    }
  } catch (e) {
    alert('网络异常');
  }
}
