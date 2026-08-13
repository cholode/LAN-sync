// src/modules/upload.js - object storage direct upload
// Backend issues a presigned URL via /files/presign; frontend PUTs to MinIO / OSS.
// No chunk, hash, or resume state is maintained on the frontend.
import { state } from '../store/index.js';
import { request } from '../api/api.js';

let isUploading = false;
let xhrUpload = null;

function makeClientMsgID() {
    if (
        globalThis.crypto &&
        typeof globalThis.crypto.randomUUID === 'function'
    ) {
        return globalThis.crypto.randomUUID();
    }
    return 'msg-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
}

function sendFileMessage(roomId, downloadUrl) {
    try {
        if (state.ws && state.ws.readyState === WebSocket.OPEN) {
            state.ws.send(JSON.stringify({
                room_id: roomId,
                content: '[文件] ' + downloadUrl,
                client_msg_id: makeClientMsgID()
            }));
        }
    } catch (e) {
        // File upload succeeded; do not surface message-send failures as upload failures.
    }
}

function updateProgress(percent) {
    const bar = document.getElementById('progress-bar');
    const status = document.getElementById('upload-status');
    if (bar) bar.style.width = Math.max(0, Math.min(100, percent)) + '%';
    if (status) status.textContent = 'upload-status' + Math.round(percent) + '%';
}

function lockUploadUI() {
    isUploading = true;
    document.getElementById('btn-upload').disabled = true;
    document.getElementById('btn-cancel').disabled = false;
    updateProgress(0);
}

function unlockUploadUI() {
    isUploading = false;
    xhrUpload = null;
    document.getElementById('btn-upload').disabled = false;
    document.getElementById('btn-cancel').disabled = true;
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

export async function startUpload() {
    if (!state.currentRoomId) return alert('请先选择群聊！');
    if (isUploading) return;

    const fileInput = document.getElementById('file-input');
    if (!fileInput.files.length) return alert('请选择文件');

    const file = fileInput.files[0];
    if (!file || file.size <= 0) return alert('空文件暂不支持上传');

    const uploadRoomId = state.currentRoomId;
    const statusBox = document.getElementById('upload-status');

    lockUploadUI();

    try {
        const res = await request('/files/presign', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                filename: file.name,
                file_type: file.name.split('.').pop() || 'file',
                file_size: file.size
            })
        });

        const data = await res.json();
        if (!res.ok) {
            throw new Error(data.error || 'Presign failed');
        }
        if (!data.upload_url || !data.object_key) {
            throw new Error('Presign response missing URL');
        }

        await putFileWithProgress(data.upload_url, file);

        statusBox.textContent = '上传完成';
        const downloadUrl = '/api/v1/download/' + encodeURIComponent(data.object_key);
        sendFileMessage(uploadRoomId, downloadUrl);
    } catch (err) {
        if (err && err.name === 'AbortError') {
            statusBox.textContent = '已取消上传';
        } else {
            statusBox.textContent = '失败: ' + (err.message || err);
        }
    } finally {
        unlockUploadUI();
    }
}

export function cancelUpload() {
    if (!isUploading) return;

    if (xhrUpload) {
        xhrUpload.abort();
        xhrUpload = null;
    }

    const statusBox = document.getElementById('upload-status');
    if (statusBox) statusBox.textContent = '已取消上传';
    updateProgress(0);
}
