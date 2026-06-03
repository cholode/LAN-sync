// src/modules/auth.js —— 登录/注册/登出
import { state, setToken, setUser, clearAuth, resetState } from '../store/index.js';
import { switchView } from '../router/index.js';
import { initChat } from './chat.js';

export async function auth(action) {
    var u = document.getElementById('username').value;
    var p = document.getElementById('password').value;
    var msgBox = document.getElementById('msg');

    if (!u || !p) return msgBox.innerText = '请输入用户名和密码';
    msgBox.innerText = '正在处理...';

    try {
        var res = await fetch(state.apiBase + '/' + action, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username: u, password: p })
        });
        var data = await res.json();

        if (res.ok && action === 'login') {
            setToken(data.token);
            if (data.user) {
                setUser(data.user);
            } else {
                setUser({ username: u });
            }
            resetState();
            msgBox.innerText = '登录成功，正在跳转...';
            setTimeout(function() {
                switchView();
                initChat();
            }, 300);
        } else if (res.ok && action === 'register') {
            msgBox.innerText = '注册成功，请登录';
        } else {
            msgBox.innerText = data.error || '操作失败';
        }
    } catch (e) {
        msgBox.innerText = '网络异常';
    }
}

export function logout() {
    clearAuth();
    cleanDirtyDOM();
    resetState();
    switchView();
}

// 清除上一用户残留在 DOM 中的脏数据（侧边栏群列表、聊天区、成员列表）
function cleanDirtyDOM() {
    var roomList = document.getElementById('room-list');
    if (roomList) roomList.innerHTML = '';

    var memberList = document.getElementById('member-list');
    if (memberList) memberList.innerHTML = '';

    var msgInput = document.getElementById('msg-content');
    if (msgInput) { msgInput.value = ''; msgInput.disabled = true; }

    var btnSend = document.getElementById('btn-send');
    if (btnSend) btnSend.disabled = true;

    var titleEl = document.getElementById('current-room-title');
    if (titleEl) titleEl.textContent = '请选择群聊';

    var subEl = document.getElementById('current-room-sub');
    if (subEl) subEl.textContent = '';

    var chatBox = document.getElementById('chat-box');
    if (chatBox) chatBox.remove();
}