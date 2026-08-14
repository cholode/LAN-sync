import { state } from '../store/index.js';

let liveMessageHandler = null;

export function onLiveMessage(handler) {
  liveMessageHandler = handler;
}

export function emitLiveMessage(message) {
  if (liveMessageHandler) {
    liveMessageHandler(message);
  }
}

export function notifyLine(text, kind = 'sys') {
  const message = {
    sender_id: null,
    content: text,
    kind: kind === 'err' ? 'err' : 'sys',
    created_at: new Date().toISOString(),
  };
  state.messages.push(message);
  emitLiveMessage(message);
}
