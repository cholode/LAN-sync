// src/modules/auth.js —— 登录/注册/登出
import { state, setToken, setUser, clearAuth } from '../store/index.js';
import { switchView } from '../router/index.js';
import { initChat } from './chat.js';

export async function auth(action) {
    const u = document.getElementById('username').value;
    const p = document.getElementById('password').value;
    const msgBox = document.getElementById('msg');

    if (!u || !p) return msgBox.innerText = '请输入用户名和密码';
    msgBox.innerText = '正在处理...';

    try {
        const res = await fetch(state.apiBase + '/' + action, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username: u, password: p })
        });
        const data = await res.json();

        if (res.ok && action === 'login') {
            setToken(data.token);
            if (data.user) {
                setUser(data.user);
            } else {
                setUser({ username: u });
            }
            msgBox.innerText = '登录成功，正在跳转...';
            setTimeout(() => {
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
    if (state.ws) {
        state.ws.close();
        state.ws = null;
    }
    switchView();
}