// Vue 版对象存储直传服务：后端签发预签名 URL，浏览器直接上传 MinIO / OSS。
import { imApi } from '../api/im.js'
let currentHashWorker = null;
const HASH_CHUNK_SIZE = 2 * 1024 * 1024;
const UPLOAD_TIMEOUT_MS = 10 * 60 * 1000;
const API_BASE = (import.meta.env.VITE_API_BASE || '/api/v1').replace(/\/$/, '');


async function fetchTimeout(url, options, timeoutMs, parentSignal) {
  const ctl=new AbortController(); let timedOut=false;
  const onAbort=()=>ctl.abort();
  if(parentSignal){ if(parentSignal.aborted) ctl.abort(); else parentSignal.addEventListener('abort',onAbort,{once:true}); }
  const timer=setTimeout(()=>{timedOut=true;ctl.abort()},timeoutMs);
  try { return await fetch(url,{...options,signal:ctl.signal}); }
  catch(e){ if(parentSignal?.aborted){const x=new Error('上传已取消');x.name='AbortError';throw x} if(timedOut){const x=new Error('请求超时');x.name='TimeoutError';throw x} throw e; }
  finally { clearTimeout(timer); parentSignal?.removeEventListener('abort',onAbort); }
}

export function computeFileSha256Hex(file, signal, onProgress) {
    return new Promise((resolve, reject) => {
        const workerScript = `
            const K = new Uint32Array([
                0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
                0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
                0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
                0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
                0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
                0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
                0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
                0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2
            ]);

            function rotr(x, n) { return (x >>> n) | (x << (32 - n)); }

            class SHA256 {
                constructor() {
                    this.h = new Uint32Array([
                        0x6a09e667,0xbb67ae85,0x3c6ef372,0xa54ff53a,
                        0x510e527f,0x9b05688c,0x1f83d9ab,0x5be0cd19
                    ]);
                    this.buf = new Uint8Array(64);
                    this.bufLen = 0;
                    this.bytesHashed = 0;
                    this.w = new Uint32Array(64);
                    this.finished = false;
                }

                _process(block, offset) {
                    const w = this.w;
                    for (let i = 0; i < 16; i++) {
                        const j = offset + i * 4;
                        w[i] = (((block[j] << 24) | (block[j+1] << 16) | (block[j+2] << 8) | block[j+3]) >>> 0);
                    }
                    for (let i = 16; i < 64; i++) {
                        const x = w[i - 15];
                        const y = w[i - 2];
                        const s0 = (rotr(x, 7) ^ rotr(x, 18) ^ (x >>> 3)) >>> 0;
                        const s1 = (rotr(y, 17) ^ rotr(y, 19) ^ (y >>> 10)) >>> 0;
                        w[i] = (w[i - 16] + s0 + w[i - 7] + s1) >>> 0;
                    }

                    let a=this.h[0], b=this.h[1], c=this.h[2], d=this.h[3];
                    let e=this.h[4], f=this.h[5], g=this.h[6], h=this.h[7];

                    for (let i = 0; i < 64; i++) {
                        const S1 = (rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)) >>> 0;
                        const ch = ((e & f) ^ (~e & g)) >>> 0;
                        const t1 = (h + S1 + ch + K[i] + w[i]) >>> 0;
                        const S0 = (rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)) >>> 0;
                        const maj = ((a & b) ^ (a & c) ^ (b & c)) >>> 0;
                        const t2 = (S0 + maj) >>> 0;
                        h=g; g=f; f=e; e=(d+t1)>>>0; d=c; c=b; b=a; a=(t1+t2)>>>0;
                    }

                    this.h[0]=(this.h[0]+a)>>>0; this.h[1]=(this.h[1]+b)>>>0;
                    this.h[2]=(this.h[2]+c)>>>0; this.h[3]=(this.h[3]+d)>>>0;
                    this.h[4]=(this.h[4]+e)>>>0; this.h[5]=(this.h[5]+f)>>>0;
                    this.h[6]=(this.h[6]+g)>>>0; this.h[7]=(this.h[7]+h)>>>0;
                }

                update(data) {
                    if (this.finished) throw new Error('SHA-256 已结束');
                    this.bytesHashed += data.length;
                    let pos = 0;

                    if (this.bufLen > 0) {
                        while (this.bufLen < 64 && pos < data.length) {
                            this.buf[this.bufLen++] = data[pos++];
                        }
                        if (this.bufLen === 64) {
                            this._process(this.buf, 0);
                            this.bufLen = 0;
                        }
                    }

                    while (pos + 64 <= data.length) {
                        this._process(data, pos);
                        pos += 64;
                    }

                    while (pos < data.length) {
                        this.buf[this.bufLen++] = data[pos++];
                    }
                    return this;
                }

                digestHex() {
                    if (this.finished) throw new Error('SHA-256 已结束');
                    this.finished = true;

                    const bitsHi = Math.floor(this.bytesHashed / 0x20000000) >>> 0;
                    const bitsLo = (this.bytesHashed << 3) >>> 0;

                    this.buf[this.bufLen++] = 0x80;
                    if (this.bufLen > 56) {
                        while (this.bufLen < 64) this.buf[this.bufLen++] = 0;
                        this._process(this.buf, 0);
                        this.bufLen = 0;
                    }
                    while (this.bufLen < 56) this.buf[this.bufLen++] = 0;

                    this.buf[56]=(bitsHi>>>24)&255; this.buf[57]=(bitsHi>>>16)&255;
                    this.buf[58]=(bitsHi>>>8)&255;  this.buf[59]=bitsHi&255;
                    this.buf[60]=(bitsLo>>>24)&255; this.buf[61]=(bitsLo>>>16)&255;
                    this.buf[62]=(bitsLo>>>8)&255;  this.buf[63]=bitsLo&255;
                    this._process(this.buf, 0);

                    return Array.from(this.h)
                        .map(v => v.toString(16).padStart(8, '0'))
                        .join('');
                }
            }

            self.onmessage = async (e) => {
                try {
                    const file = e.data.file;
                    const chunkSize = e.data.chunkSize;
                    const sha = new SHA256();
                    const chunks = Math.ceil(file.size / chunkSize);

                    for (let i = 0; i < chunks; i++) {
                        const start = i * chunkSize;
                        const end = Math.min(start + chunkSize, file.size);
                        const buf = await file.slice(start, end).arrayBuffer();
                        sha.update(new Uint8Array(buf));
                        self.postMessage({ type: 'progress', done: i + 1, total: chunks });
                    }
                    self.postMessage({ type: 'done', hash: sha.digestHex() });
                } catch (err) {
                    self.postMessage({ type: 'error', error: err && err.message ? err.message : '哈希计算异常' });
                }
            };
        `;

        const blob = new Blob([workerScript], { type: 'application/javascript' });
        const workerUrl = URL.createObjectURL(blob);
        const worker = new Worker(workerUrl);
        currentHashWorker = worker;
        let settled = false;

        const cleanup = () => {
            if (currentHashWorker === worker) currentHashWorker = null;
            worker.terminate();
            URL.revokeObjectURL(workerUrl);
            if (signal) signal.removeEventListener('abort', onAbort);
        };

        const finishReject = (err) => {
            if (settled) return;
            settled = true;
            cleanup();
            reject(err);
        };

        const onAbort = () => {
            const err = new Error('上传已取消');
            err.name = 'AbortError';
            finishReject(err);
        };

        if (signal) {
            if (signal.aborted) return onAbort();
            signal.addEventListener('abort', onAbort, { once: true });
        }

        worker.onmessage = (e) => {
            if (e.data.type === 'progress') {
                if (typeof onProgress === 'function') onProgress(e.data.done, e.data.total);
                return;
            }
            if (settled) return;
            settled = true;
            cleanup();
            if (e.data.type === 'done') resolve(e.data.hash);
            else reject(new Error(e.data.error || '哈希计算异常'));
        };

        worker.onerror = (err) => {
            finishReject(new Error('Worker 线程执行崩溃: ' + (err.message || '未知错误')));
        };

        worker.postMessage({ file, chunkSize: HASH_CHUNK_SIZE });
    });
}

async function readError(res) {
  try {
    const data = await res.clone().json()
    return data.error || data.message || data.msg || JSON.stringify(data)
  } catch {
    return await res.text().catch(() => `HTTP ${res.status}`)
  }
}

function extensionOf(filename = '') {
  const idx = filename.lastIndexOf('.')
  return idx >= 0 ? filename.slice(idx + 1).toLowerCase() : ''
}

function makeDownloadUrl(objectKey) {
  return `${API_BASE}/download/${encodeURIComponent(objectKey)}`
}

export async function uploadFile(file, { onStage = () => {}, onProgress = () => {}, signal, roomId } = {}) {
  const controller = new AbortController()
  const forward = () => controller.abort()
  signal?.addEventListener('abort', forward, { once: true })

  try {
    onStage('hash')
    const hash = await computeFileSha256Hex(file, controller.signal, (done, total) => {
      onProgress(Math.round((done / total) * 10))
    })

    onStage('presign')
    onProgress(12)
    const presign = await imApi.presignUpload({
      filename: file.name,
      file_type: extensionOf(file.name),
      file_size: file.size,
    })

    onStage('upload')
    onProgress(20)
    const uploadRes = await fetchTimeout(
      presign.upload_url,
      {
        method: 'PUT',
        headers: { 'Content-Type': file.type || 'application/octet-stream' },
        body: file,
      },
      UPLOAD_TIMEOUT_MS,
      controller.signal,
    )
    if (!uploadRes.ok) {
      throw new Error(`对象存储上传失败：HTTP ${uploadRes.status} ${await readError(uploadRes)}`)
    }

    onStage('complete')
    onProgress(90)
    const completeBody = {
      object_key: presign.object_key,
      original_name: file.name,
      sha256: hash,
      file_size: file.size,
    }
    if (roomId !== undefined && roomId !== null && Number(roomId) > 0) {
      completeBody.room_id = Number(roomId)
    }
    const record = await imApi.completeUpload(completeBody)

    onProgress(100)
    return {
      object_key: presign.object_key,
      download_url: makeDownloadUrl(presign.object_key),
      hash,
      file_name: file.name,
      file_size: file.size,
      record,
      instant: false,
    }
  } finally {
    signal?.removeEventListener('abort', forward)
  }
}