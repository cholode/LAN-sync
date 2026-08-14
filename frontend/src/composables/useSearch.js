import { state } from '../store/index.js';
import { request, readErrorMessage } from '../api/api.js';

export function resetMessageSearch() {
  state.messageSearch = {
    visible: false,
    query: '',
    results: [],
    total: 0,
    from: 0,
    loading: false,
    error: '',
  };
}

export async function searchRoomMessages(query, from = 0, size = 20) {
  if (!state.currentRoomId) {
    throw new Error('请先选择群聊');
  }

  const keyword = String(query || '').trim();
  if (!keyword) {
    throw new Error('请输入搜索关键词');
  }

  const endpoint =
    '/rooms/' +
    state.currentRoomId +
    '/messages/search?q=' +
    encodeURIComponent(keyword) +
    '&from=' +
    from +
    '&size=' +
    size;

  const res = await request(endpoint);
  if (!res.ok) {
    const message = await readErrorMessage(res);
    throw new Error(message || '搜索失败');
  }

  const data = await res.json();
  return {
    total: Number(data.total || 0),
    messages: data.messages || [],
  };
}
