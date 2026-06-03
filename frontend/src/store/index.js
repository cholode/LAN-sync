// src/store/index.js —— 全局状态存储
export const state = {
    apiBase: '/api/v1',
    wsBase: (location.protocol === 'https:' ? 'wss:' : 'ws:') + '//' + location.host + '/api/v1/ws',
    jwtToken: localStorage.getItem('lan_im_token'),
    user: JSON.parse(localStorage.getItem('lan_im_user') || 'null'),
    ws: null,
    currentRoomId: null,
    currentRoomName: '',
    loadedMessageIds: new Set(),
    myRooms: []
};

export function setToken(token) {
    state.jwtToken = token;
    localStorage.setItem('lan_im_token', token);
}

export function setUser(user) {
    state.user = user;
    localStorage.setItem('lan_im_user', JSON.stringify(user));
}

export function clearAuth() {
    state.jwtToken = null;
    state.user = null;
    localStorage.removeItem('lan_im_token');
    localStorage.removeItem('lan_im_user');
}