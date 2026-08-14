// src/modules/chat.js —— 聊天核心：WS连接、消息收发、群聊管理
import { state } from '../store/index.js';
import { request, readErrorMessage } from '../api/api.js';
import {
    notifyLine,
    insertMessageNode,
    prependMessageNode,
    isNearBottom,
    ensureChatScrollable,
    resetChatArea,
    setChatPlaceholder,
    clearChatPlaceholder,
    updateDisbandButtonState,
    escapeHtml
} from '../utils/ui.js';
import { loadAgentState } from './agent.js';

// ============ WebSocket ============

export function connectWS() {
    if (state.ws) {
        state.ws.close();
    }
    state.ws = new WebSocket(state.wsBase + '?token=' + state.jwtToken);

    state.ws.onopen = () => notifyLine('实时连接已建立', 'sys');
    state.ws.onclose = () => notifyLine('连接已断开', 'err');
    state.ws.onerror = () => notifyLine('连接异常', 'err');

    state.ws.onmessage = (e) => {
        var msgs = e.data.split('\n');
        msgs.forEach(function(rawText) {
            if (!rawText.trim()) return;
            try {
                var msg = JSON.parse(rawText);
                var room = msg.room_id != null ? msg.room_id : msg.RoomID;
                var content = msg.content != null ? msg.content : msg.Content;
                var sender = msg.sender_id != null ? msg.sender_id : msg.SenderID;

                var pseudo = {
                    sender_id: sender,
                    content: content,
                    created_at: new Date().toISOString()
                };

                var cacheKey = room != null ? String(room) : 'global';
                if (!state.messageCache[cacheKey]) {
                    state.messageCache[cacheKey] = [];
                }
                state.messageCache[cacheKey].push(pseudo);
                if (state.messageCache[cacheKey].length > state.MAX_CACHE_SIZE) {
                    state.messageCache[cacheKey].shift();
                }

                if (room === state.currentRoomId || room === '全局') {
                    var box = ensureChatScrollable();
                    var near = isNearBottom(box);
                    insertMessageNode(pseudo, 'live');
                    if (box && near) box.scrollTop = box.scrollHeight;
                }
            } catch (err) {
                notifyLine('收到无法解析的消息', 'err');
            }
        });
    };
}

export function initChat() {
    if (state.user) {
        var topName = document.getElementById('top-username');
        var topId = document.getElementById('top-userid');
        var asideName = document.getElementById('aside-username');
        var asideDetail = document.getElementById('aside-userdetail');
        var avatar = document.getElementById('aside-avatar');
        if (topName) topName.textContent = state.user.username || '用户';
        if (topId) topId.textContent = '#' + (state.user.id || state.user.user_id || '—');
        if (asideName) asideName.textContent = state.user.username || '—';
        if (asideDetail) asideDetail.textContent = 'UID: ' + (state.user.id || state.user.user_id || '—');
        if (avatar) avatar.textContent = (state.user.username || '?')[0].toUpperCase();
    }
    connectWS();
    bindHistoryScroll();
    loadMyRooms();
}

// ============ 消息发送 ============

export function sendMsg() {
    if (!state.currentRoomId) return alert('请先选择一个群聊！');
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) return alert('连接未就绪！');

    var input = document.getElementById('msg-content');
    if (!input.value) return;

    var clientMsgId = crypto.randomUUID();
    state.ws.send(JSON.stringify({
        room_id: state.currentRoomId,
        content: input.value,
        client_msg_id: clientMsgId
    }));

    input.value = '';
}

export function handleEnter(event) {
    if (event.key === 'Enter') {
        event.preventDefault();
        sendMsg();
    }
}

// ============ 群聊列表渲染 ============

export async function loadMyRooms() {
    try {
        var res = await request('/my_rooms');
        if (!res.ok) return;
        var data = await res.json();
        state.myRooms = data.rooms || data || [];
        renderRoomList();
    } catch (e) {
        console.error('加载群聊列表失败', e);
    }
}

export function renderRoomList(filterText) {
    var container = document.getElementById('room-list');
    if (!container) return;

    var rooms = state.myRooms || [];
    if (filterText) {
        var ft = filterText.toLowerCase();
        rooms = rooms.filter(function(r) {
            var name = (r.name || '').toLowerCase();
            var id = String(r.id || r.room_id || '');
            return name.includes(ft) || id.includes(ft);
        });
    }

    if (rooms.length === 0) {
        container.innerHTML = '<div style="color:#888;text-align:center;padding:20px;font-family:monospace;">暂无群聊</div>';
        return;
    }

    container.innerHTML = rooms.map(function(r) {
        var rid = r.id || r.room_id;
        var rname = escapeHtml(r.name || '未命名群聊');
        var cls = rid === state.currentRoomId ? 'room-item active' : 'room-item';
        return '<div class="' + cls + '" data-room-id="' + rid + '" onclick="window.selectRoom(' + rid + ')">' +
            '<div class="room-item-name">' + rname + '</div>' +
            '<div class="room-item-id">#' + rid + '</div>' +
            '</div>';
    }).join('');
}

export function filterRooms() {
    var searchInput = document.getElementById('room-search');
    renderRoomList(searchInput ? searchInput.value : '');
}

var historyMeta = {};

function getHistoryMeta(roomId) {
    var key = String(roomId);
    if (!historyMeta[key]) {
        historyMeta[key] = { cursor: 0, hasMore: false, loading: false };
    }
    return historyMeta[key];
}

function resetHistoryMeta(roomId) {
    historyMeta[String(roomId)] = { cursor: 0, hasMore: false, loading: false };
}

function setHistoryHint(text) {
    var hint = document.getElementById('history-hint');
    if (hint) hint.textContent = text || '';
}

export async function selectRoom(roomId) {
    if (state.currentRoomId === roomId) return;

    resetHistoryMeta(roomId);
    bindHistoryScroll();

    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        try {
            await request('/rooms/' + roomId + '/join', { method: 'POST' });
        } catch (e) {}
    }

    state.currentRoomId = roomId;
    var room = (state.myRooms || []).find(function(r) { return (r.id || r.room_id) === roomId; });
    state.currentRoomName = room ? room.name : '群聊 #' + roomId;

    var titleEl = document.getElementById('current-room-title');
    var subEl = document.getElementById('current-room-sub');
    if (titleEl) titleEl.textContent = state.currentRoomName;
    if (subEl) subEl.textContent = 'ID: ' + roomId;

    document.getElementById('msg-content').disabled = false;
    document.getElementById('btn-send').disabled = false;
    updateDisbandButtonState();

    resetChatArea();

    var cacheKey = String(roomId);
    var cached = state.messageCache[cacheKey] || [];
    if (cached.length > 0) {
        clearChatPlaceholder();
        cached.forEach(function(m) { insertMessageNode(m, 'live'); });
        var box = ensureChatScrollable();
        if (box) box.scrollTop = box.scrollHeight;
    } else {
        setChatPlaceholder('正在加载消息...');
    }

    renderRoomList();
    loadRoomMembers(roomId);
    loadAgentState();

    await loadHistory(roomId);
}

// ============ 历史消息 ============

export async function loadHistory(roomId, prepend) {
    var meta = getHistoryMeta(roomId);
    if (meta.loading) return;
    meta.loading = true;

    var cursor = prepend ? meta.cursor : 0;
    var endpoint = '/rooms/' + roomId + '/messages?limit=50&cursor=' + cursor;

    try {
        if (prepend) setHistoryHint('加载更多历史消息...');
        else setHistoryHint('正在加载历史消息...');

        var res = await request(endpoint);
        if (!res.ok) {
            var cacheKey = String(roomId);
            if (!prepend && (!state.messageCache[cacheKey] || state.messageCache[cacheKey].length === 0)) {
                setChatPlaceholder('加载失败 (HTTP ' + res.status + ')，请重试');
            }
            setHistoryHint('');
            return;
        }

        var data = await res.json();
        var messages = data.messages || data || [];
        var nextCursor = data.next_cursor ? Number(data.next_cursor) : 0;
        var hasMore = data.has_more === true;

        if (messages.length === 0) {
            meta.cursor = 0;
            meta.hasMore = false;
            if (!prepend) {
                var cacheKey2 = String(roomId);
                if (!state.messageCache[cacheKey2] || state.messageCache[cacheKey2].length === 0) {
                    setChatPlaceholder('暂无消息，发送第一条吧');
                }
            }
            setHistoryHint('没有更多历史消息');
            return;
        }

        meta.cursor = nextCursor || 0;
        meta.hasMore = hasMore;

        if (!prepend) {
            var nearBefore = isNearBottom(ensureChatScrollable());
            resetChatArea();
            messages.forEach(function(msg) { insertMessageNode(msg, 'hist'); });

            var box = ensureChatScrollable();
            if (nearBefore && box) box.scrollTop = box.scrollHeight;
        } else {
            var box2 = ensureChatScrollable();
            var oldHeight = box2 ? box2.scrollHeight : 0;
            var oldTop = box2 ? box2.scrollTop : 0;

            messages.forEach(function(msg) { prependMessageNode(msg, 'hist'); });

            if (box2) box2.scrollTop = oldTop + (box2.scrollHeight - oldHeight);
        }

        setHistoryHint(hasMore ? '向上滚动加载更多历史消息' : '已加载全部历史消息');
    } catch (e) {
        var cacheKey3 = String(roomId);
        if (!prepend && (!state.messageCache[cacheKey3] || state.messageCache[cacheKey3].length === 0)) {
            setChatPlaceholder('加载失败：' + (e.message || '网络异常'));
        }
        setHistoryHint('');
    } finally {
        meta.loading = false;
    }
}

export async function loadMoreHistory() {
    if (!state.currentRoomId) return;
    var meta = getHistoryMeta(state.currentRoomId);
    if (!meta.hasMore || meta.loading) return;
    await loadHistory(state.currentRoomId, true);
}

function bindHistoryScroll() {
    var wrap = document.getElementById('chat-wrap');
    if (!wrap || wrap.dataset.historyBound) return;
    wrap.dataset.historyBound = '1';
    wrap.addEventListener('scroll', function() {
        if (!state.currentRoomId) return;
        if (wrap.scrollTop <= 40) {
            loadMoreHistory();
        }
    });
}


export async function loadRoomMembers(roomId) {
    var list = document.getElementById('member-list');
    var hint = document.getElementById('members-room-hint');
    if (!list) return;
    try {
        var res = await request('/rooms/' + roomId + '/members');
        if (!res.ok) {
            list.innerHTML = '<div style="color:#888;font-size:11px;padding:8px 12px;">加载失败</div>';
            return;
        }
        var data = await res.json();
        var members = data.members || data || [];
        if (hint) hint.textContent = '群聊 #' + roomId + ' · ' + members.length + ' 人';
        if (members.length === 0) {
            list.innerHTML = '<div style="color:#888;font-size:11px;padding:8px 12px;">暂无成员</div>';
            return;
        }
        list.innerHTML = members.map(function(m) {
            var uid = m.user_id || m.id;
            var uname = escapeHtml(m.username || '用户');
            var isCreator = m.is_creator === true;
            var role = (m.role != null) ? Number(m.role) : 1;
            var badge = '';
            if (isCreator || role === 3) {
                badge = ' <span class="member-badge owner">群主</span>';
            } else if (role === 2) {
                badge = ' <span class="member-badge manager">管理</span>';
            }
            return '<div class="member-item">' +
                '<span class="member-name">' + uname + badge + '</span>' +
                '<span class="member-id">#' + uid + '</span>' +
                '</div>';
        }).join('');
    } catch (e) {
        list.innerHTML = '<div style="color:#888;font-size:11px;padding:8px 12px;">加载失败</div>';
    }
}

// ============ 创建/加入/退出/解散 ============

export async function createRoom() {
    var name = document.getElementById('new-room-name').value.trim();
    if (!name) return alert('群聊名称不可为空');
    try {
        var res = await request('/rooms', {
            method: 'POST',
            body: JSON.stringify({ name: name })
        });
        if (res.ok) {
            document.getElementById('new-room-name').value = '';
            await loadMyRooms();
        } else {
            var err = await readErrorMessage(res);
            alert('创建群聊失败: ' + err);
        }
    } catch (e) {
        alert('网络异常');
    }
}

export async function joinRoom() {
    var id = parseInt(document.getElementById('join-room-id').value, 10);
    if (!id) return alert('请输入群聊ID');
    try {
        var res = await request('/rooms/' + id + '/join', { method: 'POST' });
        if (res.ok) {
            document.getElementById('join-room-id').value = '';
            await loadMyRooms();
        } else {
            var err = await readErrorMessage(res);
            alert('加入群聊失败: ' + err);
        }
    } catch (e) {
        alert('网络异常');
    }
}

export async function removeMember() {
    if (!state.currentRoomId) return alert('请先选择群聊');
    var uidInput = document.getElementById('kick-user-id');
    var targetId = parseInt(uidInput.value, 10);
    if (!targetId) return alert('请输入要移除的用户ID');

    var selfId = state.user ? (state.user.id || state.user.user_id) : null;
    var isSelfKick = selfId != null && targetId === Number(selfId);

    if (isSelfKick && !confirm('确定退出当前群聊？')) return;
    if (!isSelfKick && !confirm('确定移除用户 #' + targetId + '？')) return;

    try {
        var res = await request('/rooms/' + state.currentRoomId + '/members/' + targetId, {
            method: 'DELETE'
        });
        if (res.ok) {
            uidInput.value = '';
            if (isSelfKick) {
                state.currentRoomId = null;
                state.currentRoomName = '';
                document.getElementById('current-room-title').textContent = '请选择群聊';
                document.getElementById('current-room-sub').textContent = '';
                document.getElementById('msg-content').disabled = true;
                document.getElementById('btn-send').disabled = true;
                resetChatArea();
            }
            await loadMyRooms();
            if (state.currentRoomId) await loadRoomMembers(state.currentRoomId);
        } else {
            var err = await readErrorMessage(res);
            alert('操作失败: ' + err);
        }
    } catch (e) {
        alert('网络异常');
    }
}

export async function disbandCurrentRoom() {
    if (!state.currentRoomId) return;
    if (!confirm('警告：解散群聊后不可恢复！确定解散 #' + state.currentRoomId + '？')) return;

    try {
        var res = await request('/rooms/' + state.currentRoomId + '/disband', {
            method: 'DELETE'
        });
        if (res.ok) {
            state.currentRoomId = null;
            state.currentRoomName = '';
            document.getElementById('current-room-title').textContent = '请选择群聊';
            document.getElementById('current-room-sub').textContent = '';
            document.getElementById('msg-content').disabled = true;
            document.getElementById('btn-send').disabled = true;
            document.getElementById('member-list').innerHTML = '<div style="color:#888;font-size:11px;padding:8px 12px;">暂无成员</div>';
            resetChatArea();
            await loadMyRooms();
        } else {
            var err = await readErrorMessage(res);
            alert('解散失败: ' + err);
        }
    } catch (e) {
        alert('网络异常');
    }
}
