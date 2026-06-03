// src/router/index.js —— SPA 视图切换引擎
import { state } from '../store/index.js';

export function switchView() {
    const token = localStorage.getItem('lan_im_token');
    const loginView = document.getElementById('login-view');
    const chatView = document.getElementById('chat-view');

    if (!loginView || !chatView) {
        console.error('[致命异常] 路由引擎寻址失败，DOM 树中缺少核心视图容器。');
        return;
    }

    if (token) {
        loginView.style.display = 'none';
        chatView.style.display = 'flex';
        console.log('[路由总机] 鉴权通过，已挂载主业务视图。');
    } else {
        loginView.style.display = 'flex';
        chatView.style.display = 'none';
        if (state.ws) {
            state.ws.close();
            state.ws = null;
            console.log('[路由总机] 视图降级，已物理斩断残存 WebSocket 连接。');
        }
    }
}
