// src/api/api.js —— HTTP 请求封装 & 全局拦截
import { state, clearAuth } from '../store/index.js';

export async function readErrorMessage(res) {
  try {
    const data = await res.clone().json();
    if (data && typeof data.error === 'string' && data.error) return data.error;
    if (data && typeof data.msg === 'string' && data.msg) return data.msg;
  } catch (e) {}

  try {
    const text = await res.text();
    if (text) return text.slice(0, 180);
  } catch (e) {}

  return 'HTTP ' + res.status;
}

export async function request(endpoint, options = {}) {
  const url = state.apiBase + endpoint;
  const headers = { ...options.headers };

  if (state.jwtToken) {
    headers['Authorization'] = 'Bearer ' + state.jwtToken;
  }
  if (!headers['Content-Type'] && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }

  const config = { ...options, headers };

  try {
    const res = await fetch(url, config);
    if (res.status === 401) {
      clearAuth();
      throw new Error('认证凭证已失效');
    }
    return res;
  } catch (err) {
    throw err;
  }
}
