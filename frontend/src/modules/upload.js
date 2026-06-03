// src/modules/upload.js —— 文件分片上传（使用浏览器原生 crypto.subtle）
import { state } from '../store/index.js';
import { request, readErrorMessage } from '../api/api.js';

var isUploading = false;
var uploadAbortController = null;
var currentUploadFileHash = null;

// 使用浏览器原生 Web Crypto API 计算 SHA-256，无需任何外部依赖
export async function computeFileSha256Hex(file) {
    var fullBuf = await file.arrayBuffer();
    var hash = await crypto.subtle.digest('SHA-256', fullBuf);
    var hexArr = Array.from(new Uint8Array(hash));
    return hexArr.map(function(b) { return b.toString(16).padStart(2, '0'); }).join('');
}

export async function startUpload() {
    if (!state.currentRoomId) return alert('请先选择群聊！');
    if (isUploading) return;

    var fileInput = document.getElementById('file-input');
    if (!fileInput.files.length) return alert('请选择文件');

    var file = fileInput.files[0];
    if (!file || file.size <= 0) return alert('空文件暂不支持上传');

    var chunkSize = 1024 * 1024;
    var totalChunks = Math.ceil(file.size / chunkSize);
    var safeFileName = encodeURIComponent(file.name);
    var statusBox = document.getElementById('upload-status');

    statusBox.textContent = '[0/3] 计算整文件 SHA-256...';
    var fileHash;
    try {
        fileHash = await computeFileSha256Hex(file);
    } catch (e) {
        statusBox.textContent = '校验和失败: ' + (e.message || e);
        return;
    }
    currentUploadFileHash = fileHash;

    isUploading = true;
    document.getElementById('btn-upload').disabled = true;
    document.getElementById('btn-cancel').disabled = false;
    uploadAbortController = new AbortController();

    try {
        statusBox.textContent = '[1/3] 检查秒传状态...';

        var checkRes = await request('/upload/status?hash=' + fileHash + '&filename=' + safeFileName);
        var checkData = await checkRes.json();

        if (checkData.status === 'completed') {
            updateProgress(totalChunks, totalChunks);
            statusBox.textContent = '秒传成功';
            state.ws.send(JSON.stringify({
                room_id: state.currentRoomId,
                content: '[文件] ' + checkData.download_url,
                client_msg_id: crypto.randomUUID()
            }));
            unlockUploadUI();
            return;
        }

        var uploadedChunks = checkData.uploaded_chunks || [];
        var completedCount = uploadedChunks.length;
        updateProgress(completedCount, totalChunks);

        statusBox.textContent = '[2/3] 上传分片 (并发执行)...';
        var uploadTasks = [];

        for (var i = 0; i < totalChunks; i++) {
            if (uploadedChunks.includes(i)) continue;
            uploadTasks.push((function(chunkIdx) {
                return async function() {
                    var formData = new FormData();
                    formData.append('chunk', file.slice(chunkIdx * chunkSize, Math.min((chunkIdx + 1) * chunkSize, file.size)));
                    formData.append('hash', fileHash);
                    formData.append('chunk_index', String(chunkIdx));

                    var lastErr = '';
                    var maxAttempts = 2;
                    for (var attempt = 1; attempt <= maxAttempts; attempt++) {
                        var res = await fetch(state.apiBase + '/upload/chunk', {
                            method: 'POST',
                            headers: { Authorization: 'Bearer ' + state.jwtToken },
                            body: formData,
                            signal: uploadAbortController.signal
                        });
                        if (res.ok) { lastErr = ''; break; }
                        var detail = await readErrorMessage(res);
                        lastErr = 'HTTP ' + res.status + ': ' + detail;
                        if (res.status === 401) throw new Error('凭证失效');
                        if (attempt < maxAttempts) await new Promise(function(r) { setTimeout(r, 250); });
                    }
                    if (lastErr) throw new Error('分片 ' + chunkIdx + ' 上传失败：' + lastErr);
                    completedCount++;
                    updateProgress(completedCount, totalChunks);
                };
            })(i));
        }

        var MAX_CONCURRENT = 6;
        var taskIndex = 0;
        await Promise.all(
            new Array(MAX_CONCURRENT).fill(null).map(async function() {
                while (taskIndex < uploadTasks.length) {
                    var idx = taskIndex++;
                    await uploadTasks[idx]();
                }
            })
        );

        statusBox.textContent = '[3/3] 合并文件指令下发...';
        var mergeData = new FormData();
        mergeData.append('hash', fileHash);
        mergeData.append('filename', file.name);
        mergeData.append('total_chunks', String(totalChunks));

        var mergeRes = await request('/upload/merge', {
            method: 'POST',
            body: mergeData
        });
        var mergeResult = await mergeRes.json();

        statusBox.textContent = '上传完成';
        state.ws.send(JSON.stringify({
            room_id: state.currentRoomId,
            content: '[文件] ' + mergeResult.download_url,
            client_msg_id: crypto.randomUUID()
        }));
    } catch (err) {
        if (err.name === 'AbortError') {
            statusBox.textContent = '已取消上传';
        } else {
            statusBox.textContent = '失败: ' + err.message;
        }
    } finally {
        unlockUploadUI();
    }
}

export async function cancelUpload() {
    if (!isUploading) return;
    var hashToCancel = currentUploadFileHash;
    if (uploadAbortController) uploadAbortController.abort();

    var statusBox = document.getElementById('upload-status');
    if (!hashToCancel) {
        statusBox.textContent = '无法清理临时文件（缺少哈希）';
        return;
    }
    try {
        await request('/upload/cancel?hash=' + hashToCancel, { method: 'DELETE' });
        statusBox.textContent = '已请求清理临时分片';
        document.getElementById('progress-bar').style.width = '0%';
    } catch (e) {
        statusBox.textContent = '清理请求失败';
    }
}

function updateProgress(completed, total) {
    if (total <= 0) return;
    var percent = Math.round((completed / total) * 100);
    document.getElementById('progress-bar').style.width = percent + '%';
    document.getElementById('upload-status').textContent = '上传进度 ' + percent + '%';
}

function unlockUploadUI() {
    isUploading = false;
    currentUploadFileHash = null;
    document.getElementById('btn-upload').disabled = false;
    document.getElementById('btn-cancel').disabled = true;
}