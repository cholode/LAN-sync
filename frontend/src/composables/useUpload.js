import { state } from '../store/index.js';
import { request } from '../api/api.js';

let xhrUpload = null;

function makeClientMsgID() {
  if (globalThis.crypto && typeof globalThis.crypto.randomUUID === 'function') {
    return globalThis.crypto.randomUUID();
  }
  return 'msg-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
}

function sendFileMessage(roomId, downloadUrl) {
  try {
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
      state.ws.send(
        JSON.stringify({
          room_id: roomId,
          content: '[文件] ' + downloadUrl,
          client_msg_id: makeClientMsgID(),
        }),
      );
    }
  } catch (e) {
    // 文件上传已成功，不把消息发送失败当成上传失败。
  }
}

function updateProgress(percent) {
  state.upload.progress = Math.max(0, Math.min(100, percent));
}

async function computeFileSha256Hex(file) {
  if (!globalThis.crypto || !globalThis.crypto.subtle) return '';
  try {
    const buffer = await file.arrayBuffer();
    const digest = await globalThis.crypto.subtle.digest('SHA-256', buffer);
    return Array.from(new Uint8Array(digest))
      .map((byte) => byte.toString(16).padStart(2, '0'))
      .join('');
  } catch (err) {
    return '';
  }
}

function putFileWithProgress(url, file) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhrUpload = xhr;

    xhr.open('PUT', url, true);
    xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream');

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        updateProgress((event.loaded / event.total) * 100);
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
        return;
      }
      reject(new Error('HTTP ' + xhr.status));
    };

    xhr.onerror = () => reject(new Error('Network error'));
    xhr.onabort = () => {
      const err = new Error('Aborted');
      err.name = 'AbortError';
      reject(err);
    };

    xhr.send(file);
  });
}

export async function startUpload(file) {
  if (!state.currentRoomId) return alert('请先选择群聊！');
  if (state.upload.isUploading) return;
  if (!file) return alert('请选择文件');
  if (file.size <= 0) return alert('空文件暂不支持上传');

  const uploadRoomId = state.currentRoomId;
  state.upload.isUploading = true;
  state.upload.progress = 0;
  state.upload.status = '上传中 0%';

  try {
    const res = await request('/files/presign', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        filename: file.name,
        file_type: file.name.split('.').pop() || 'file',
        file_size: file.size,
      }),
    });

    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || 'Presign failed');
    }
    if (!data.upload_url || !data.object_key) {
      throw new Error('Presign response missing URL');
    }

    await putFileWithProgress(data.upload_url, file);

    state.upload.status = '上传完成';
    const downloadUrl = '/api/v1/download/' + encodeURIComponent(data.object_key);
    const sha256 = await computeFileSha256Hex(file);
    try {
      await request('/files/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          object_key: data.object_key,
          original_name: file.name,
          sha256,
          file_size: file.size,
          room_id: uploadRoomId,
        }),
      });
    } catch (err) {
      // 元数据记录失败不阻塞消息发送。
    }
    sendFileMessage(uploadRoomId, downloadUrl);
  } catch (err) {
    if (err && err.name === 'AbortError') {
      state.upload.status = '已取消上传';
    } else {
      state.upload.status = '失败: ' + (err.message || err);
    }
  } finally {
    state.upload.isUploading = false;
    xhrUpload = null;
  }
}

export function cancelUpload() {
  if (!state.upload.isUploading) return;

  if (xhrUpload) {
    xhrUpload.abort();
    xhrUpload = null;
  }

  state.upload.status = '已取消上传';
  state.upload.progress = 0;
}
