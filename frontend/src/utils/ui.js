// src/utils/ui.js —— DOM 操作 & 消息渲染工具
import { state } from '../store/index.js';

export function escapeHtml(s) {
    if (s == null) return '';
    var d = document.createElement('div');
    d.textContent = String(s);
    return d.innerHTML;
}

export function formatChatMessageHtml(raw) {
    if (raw == null) return '';
    var s = String(raw);
    var re = /(https?:\/\/[^\s]+\/api\/v1\/download\/[^\s?#]+|\/api\/v1\/download\/[^\s?#]+)/g;
    var last = 0;
    var m;
    var parts = [];
    while ((m = re.exec(s)) !== null) {
        parts.push(escapeHtml(s.slice(last, m.index)));
        var urlStr = m[1];
        var href = urlStr;
        var saveAs = '';
        try {
            var u = new URL(urlStr, window.location.origin);
            if (u.pathname.indexOf('/api/v1/download/') !== -1) {
                u.search = '';
                u.hash = '';
                href = u.href;
                var seg = u.pathname.split('/').pop() || '';
                var hm = /^([a-f0-9]{64})_(.+)$/i.exec(seg);
                saveAs = hm ? hm[2] : seg;
            }
        } catch (e) {}
        var dlAttr = saveAs ? ' download="' + escapeHtml(saveAs) + '"' : '';
        parts.push('<a class="msg-link" href="' + escapeHtml(href) + '"' + dlAttr + '>' + escapeHtml(urlStr) + '</a>');
        last = m.index + m[0].length;
    }
    parts.push(escapeHtml(s.slice(last)));
    return parts.join('');
}

// ===== DOM 工具 =====
export function getChatBox() {
    var box = document.getElementById('chat-box');
    if (!box) {
        var wrap = document.getElementById('chat-wrap');
        box = document.createElement('div');
        box.id = 'chat-box';
        wrap.appendChild(box);
    }
    return box;
}

export function ensureChatScrollable() {
    var box = document.getElementById('chat-box');
    if (!box) return null;
    return box;
}

export function isNearBottom(el) {
    if (!el) return true;
    return el.scrollHeight - el.scrollTop - el.clientHeight < 100;
}

// ===== 占位文本管理 =====
export function setChatPlaceholder(text) {
    var box = getChatBox();
    box.innerHTML = '<div class="chat-placeholder">' + escapeHtml(text) + '</div>';
}

export function clearChatPlaceholder() {
    var box = document.getElementById('chat-box');
    if (box) {
        // 只移除占位符，保留消息节点
        var ph = box.querySelector('.chat-placeholder');
        if (ph) ph.remove();
    }
}

// ===== 消息气泡渲染 =====
export function appendBubble(html, className, scrollToEnd) {
    var box = getChatBox();
    clearChatPlaceholder();
    var wrap = document.createElement('div');
    wrap.className = 'msg-row ' + (className || 'other');
    wrap.innerHTML = html;
    box.appendChild(wrap);
    if (scrollToEnd !== false && isNearBottom(box)) {
        box.scrollTop = box.scrollHeight;
    }
}

export function notifyLine(text, kind) {
    var k = kind === 'err' ? 'err' : 'sys';
    appendBubble('<div class="msg-bubble">' + escapeHtml(text) + '</div>', k, true);
}

export function renderMessageRecord(msg, kind) {
    var sid = msg.sender_id != null ? msg.sender_id : msg.SenderID;
    var content = msg.content != null ? msg.content : msg.Content;
    var created = msg.created_at || msg.CreatedAt;
    var timeStr = '';
    if (created) {
        try { timeStr = new Date(created).toLocaleString(); } catch (e) {}
    }
    var meta = 'UID ' + sid + (timeStr ? ' · ' + timeStr : '');
    return '<div class="msg-meta">' + escapeHtml(meta) + '</div><div class="msg-bubble">' + formatChatMessageHtml(content) + '</div>';
}

export function insertMessageNode(msg, kind) {
    var id = msg.id != null ? msg.id : msg.ID;
    if (id != null && state.loadedMessageIds.has(String(id))) return false;
    if (id != null) state.loadedMessageIds.add(String(id));

    var box = getChatBox();
    clearChatPlaceholder();
    var wrap = document.createElement('div');
    var sid = msg.sender_id != null ? msg.sender_id : msg.SenderID;
    var selfUserId = state.user ? Number(state.user.id || state.user.user_id) : null;
    var own = selfUserId != null && Number(sid) === selfUserId;
    wrap.className = 'msg-row ' + (own ? 'own' : 'other') + (kind === 'hist' ? ' hist' : '');
    wrap.innerHTML = renderMessageRecord(msg, kind);
    box.appendChild(wrap);
    return true;
}

export function resetChatArea() {
    state.loadedMessageIds.clear();
    var hint = document.getElementById('history-hint');
    if (hint) hint.textContent = '';
    var box = document.getElementById('chat-box');
    if (box) box.remove();
    box = document.createElement('div');
    box.id = 'chat-box';
    var wrap = document.getElementById('chat-wrap');
    wrap.appendChild(box);
    setChatPlaceholder('请选择群聊开始聊天');
}

export function updateDisbandButtonState() {
    var btnDisband = document.getElementById('btn-disband-room');
    if (btnDisband) {
        btnDisband.disabled = false;
    }
}