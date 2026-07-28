// src/main.js —— SPA 入口：挂载全局函数 & 点火
import { switchView } from './router/index.js';
import { auth, logout } from './modules/auth.js';
import { sendMsg, handleEnter, createRoom, joinRoom, filterRooms, selectRoom, removeMember, disbandCurrentRoom, initChat } from './modules/chat.js';
import { startUpload, cancelUpload } from './modules/upload.js';
import { toggleAgent, removeAgent, markAgentDirty, saveAgentConfig, loadAgentState } from './modules/agent.js';

window.auth = auth;
window.logout = logout;
window.sendMsg = sendMsg;
window.handleEnter = handleEnter;
window.createRoom = createRoom;
window.joinRoom = joinRoom;
window.filterRooms = filterRooms;
window.selectRoom = selectRoom;
window.removeMember = removeMember;
window.disbandCurrentRoom = disbandCurrentRoom;
window.startUpload = startUpload;
window.cancelUpload = cancelUpload;
window.toggleAgent = toggleAgent;
window.removeAgent = removeAgent;
window.markAgentDirty = markAgentDirty;
window.saveAgentConfig = saveAgentConfig;

window.addEventListener('DOMContentLoaded', () => {
    console.log('[系统基建] LAN-IM SPA 引擎点火...');
    switchView();
    var token = localStorage.getItem('lan_im_token');
    if (token) {
        initChat();
    }
});