// src/modules/upload.js —— 文件分片上传（断点续传 + Web Worker 哈希 + 原生 crypto.subtle）
import { state } from '../store/index.js';
import { request, readErrorMessage } from '../api/api.js';

var isUploading = false;
var uploadAbortController = null;
var currentUploadFileHash = null;

// 流式哈希计算引擎 (Web Worker + 2MB 分块 + 原生 crypto.subtle)
export async function computeFileSha256Hex(file) {
    return new Promise(function(resolve, reject) {
        var workerScript = '\n\
            self.onmessage = async function(e) {\n\
                try {\n\
                    var file = e.data.file;\n\
                    var chunkSize = 2 * 1024 * 1024; // 每次仅读取 2MB\n\
                    var chunks = Math.ceil(file.size / chunkSize);\n\
                    var parts = [];\n\
                    for (var i = 0; i < chunks; i++) {\n\
                        var start = i * chunkSize;\n\
                        var end = Math.min(start + chunkSize, file.size);\n\
                        var buf = await file.slice(start, end).arrayBuffer();\n\
                        parts.push(new Uint8Array(buf));\n\
                    }\n\
                    // 拼接所有分块为完整缓冲区\n\
                    var totalLen = 0;\n\
                    for (var j = 0; j < parts.length; j++) totalLen += parts[j].byteLength;\n\
                    var result = new Uint8Array(totalLen);\n\
                    var offset = 0;\n\
                    for (var k = 0; k < parts.length; k++) {\n\
                        result.set(parts[k], offset);\n\
                        offset += parts[k].byteLength;\n\
                    }\n\
                    var hash = await crypto.subtle.digest("SHA-256", result);\n\
                    var hex = Array.from(new Uint8Array(hash)).map(function(b) { return b.toString(16).padStart(2, "0"); }).join("");\n\
                    self.postMessage({ type: "done", hash: hex });\n\
                } catch (err) {\n\
                    self.postMessage({ type: "error", error: err.message || "哈希计算异常" });\n\
                }\n\
            };\n\
        ';

        var blob = new Blob([workerScript], { type: 'application/javascript' });
        var workerUrl = URL.createObjectURL(blob);
        var worker = new Worker(workerUrl);

        worker.onmessage = function(e) {
            worker.terminate();
            URL.revokeObjectURL(workerUrl);
            if (e.data.type === 'done') {
                resolve(e.data.hash);
            } else {
                reject(new Error(e.data.error));
            }
        };

        worker.onerror = function(err) {
            worker.terminate();
            URL.revokeObjectURL(workerUrl);
            reject(new Error('Worker 线程执行崩溃: ' + err.message));
        };

        // 发送文件句柄——只传引用，不拷贝文件本体
        worker.postMessage({ file: file });
    });
}


// ---------- Presigned URL 直传（MinIO 场景） ----------

async function tryPresignedUpload(file) {
    var fileName = encodeURIComponent(file.name);
    var ext = (file.name.split('.').pop() || '').toLowerCase();

    var presignRes = await request('/files/presign', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            filename: fileName,
            file_type: ext,
            file_size: file.size
        })
    });

    if (!presignRes.ok) return null; // 后端不支持预签名，降级为分片上传

    var presignData = await presignRes.json();
    if (!presignData.upload_url) return null;

    return presignData;
}

async function uploadViaPresignedURL(file, presignData) {
    return new Promise(function(resolve, reject) {
        var xhr = new XMLHttpRequest();
        xhr.open('PUT', presignData.upload_url, true);

        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                var percent = Math.round((e.loaded / e.total) * 100);
                document.getElementById('progress-bar').style.width = percent + '%';
                document.getElementById('upload-status').textContent = 'MinIO 直传 ' + percent + '%';
            }
        };

        xhr.onload = function() {
            if (xhr.status >= 200 && xhr.status < 300) {
                resolve({ object_key: presignData.object_key });
            } else {
                reject(new Error('MinIO 上传失败: HTTP ' + xhr.status));
            }
        };

        xhr.onerror = function() {
            reject(new Error('MinIO 网络错误'));
        };

        xhr.ontimeout = function() {
            reject(new Error('MinIO 上传超时'));
        };

        xhr.timeout = 600000; // 10 分钟
        xhr.send(file);
    });
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
        statusBox.textContent = '[1/3] 检查上传状态...';
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

        statusBox.textContent = '[2/3] 上传分片...';
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
                        if (res.status === 401) throw new Error('登录已过期，请重新登录');
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

        // 合并参数本地校验（防止空哈希/空文件名）
        statusBox.textContent = '[3/3] 合并文件指令下发...';
        var mergeHash = String(fileHash || '').trim();
        var mergeFilename = String(file.name || '').trim();
        var mergeTotalChunks = String(totalChunks);
        if (!mergeHash || !mergeFilename || totalChunks <= 0) {
            throw new Error('合并参数本地校验失败');
        }

        var mergeData = new FormData();
        mergeData.append('hash', mergeHash);
        mergeData.append('filename', mergeFilename);
        mergeData.append('total_chunks', mergeTotalChunks);

        var mergeRes = await request('/upload/merge', {
            method: 'POST',
            body: mergeData
        });
        var mergeResult = await mergeRes.json();
        if (!mergeRes.ok) throw new Error(mergeResult.error || '合并失败');

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