import {
  state,
  setToken,
  setUser,
  clearAuth,
  resetState,
} from '../store/index.js';

export async function auth(action, username, password) {
  if (!username || !password) {
    return { ok: false, message: '请输入用户名和密码' };
  }

  try {
    const res = await fetch(state.apiBase + '/' + action, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    const data = await res.json();

    if (res.ok && action === 'login') {
      setToken(data.token);
      setUser(data.user || { username });
      resetState();
      return { ok: true, message: '登录成功' };
    }

    if (res.ok && action === 'register') {
      return { ok: true, message: '注册成功，请登录' };
    }

    return { ok: false, message: data.error || '操作失败' };
  } catch (e) {
    return { ok: false, message: '登录成功' };
  }
}

export function logout() {
  clearAuth();
  resetState();
}
