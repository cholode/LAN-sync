import { computed, reactive } from 'vue';

const storedToken = localStorage.getItem('lan_im_token');
const storedUser = JSON.parse(localStorage.getItem('lan_im_user') || 'null');

export const state = reactive({
  apiBase: '/api/v1',
  wsBase:
    (location.protocol === 'https:' ? 'wss:' : 'ws:') +
    '//' +
    location.host +
    '/api/v1/ws',
  jwtToken: storedToken,
  user: storedUser,
  ws: null,
  currentRoomId: null,
  currentRoomName: '',
  rooms: [],
  roomFilter: '',
  messageCache: {},
  messages: [],
  loadedMessageIds: new Set(),
  MAX_CACHE_SIZE: 500,
  historyHint: '',
  historyMeta: {},
  members: [],
  membersHint: '请选择群聊查看成员',
  newRoomName: '',
  joinRoomId: '',
  kickUserId: '',
  agent: {
    visible: false,
    configVisible: false,
    enabled: false,
    dirty: false,
    config: {
      trigger_mode: 1,
      trigger_words: '[]',
      system_prompt: '',
      max_history: 20,
      model_name: 'deepseek-chat',
      top_k: 5,
    },
  },
  upload: {
    isUploading: false,
    progress: 0,
    status: '等待上传',
  },
  messageSearch: {
    visible: false,
    query: '',
    results: [],
    total: 0,
    from: 0,
    loading: false,
    error: '',
  },
});

export const filteredRooms = computed(() => {
  const keyword = state.roomFilter.trim().toLowerCase();
  if (!keyword) return state.rooms;

  return state.rooms.filter((room) => {
    const name = String(room.name || '').toLowerCase();
    const id = String(room.id || room.room_id || '');
    return name.includes(keyword) || id.includes(keyword);
  });
});

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

export function resetState() {
  state.rooms = [];
  state.roomFilter = '';
  state.currentRoomId = null;
  state.currentRoomName = '';
  state.messageCache = {};
  state.messages = [];
  state.loadedMessageIds.clear();
  state.historyHint = '';
  state.historyMeta = {};
  state.members = [];
  state.membersHint = '请选择群聊查看成员';
  state.newRoomName = '';
  state.joinRoomId = '';
  state.kickUserId = '';
  state.agent.visible = false;
  state.agent.configVisible = false;
  state.agent.enabled = false;
  state.agent.dirty = false;
  state.agent.config = {
    trigger_mode: 1,
    trigger_words: '[]',
    system_prompt: '',
    max_history: 20,
    model_name: 'deepseek-chat',
    top_k: 5,
  };
  state.upload.isUploading = false;
  state.upload.progress = 0;
  state.upload.status = '等待上传';
  state.messageSearch = {
    visible: false,
    query: '',
    results: [],
    total: 0,
    from: 0,
    loading: false,
    error: '',
  };

  if (state.ws) {
    state.ws.onopen = null;
    state.ws.onmessage = null;
    state.ws.onclose = null;
    state.ws.onerror = null;
    state.ws.close();
    state.ws = null;
  }
}
