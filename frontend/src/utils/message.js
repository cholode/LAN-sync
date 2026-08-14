// 消息内容格式化：转义普通文本，并把下载链接转换为可点击链接。
export function escapeHtml(value) {
  if (value == null) return '';
  const div = document.createElement('div');
  div.textContent = String(value);
  return div.innerHTML;
}

export function formatChatMessageHtml(raw) {
  if (raw == null) return '';
  const source = String(raw);
  const downloadPattern =
    /(https?:\/\/[^\s]+\/api\/v1\/download\/[^\s?#]+|\/api\/v1\/download\/[^\s?#]+)/g;

  const parts = [];
  let lastIndex = 0;
  let match;

  while ((match = downloadPattern.exec(source)) !== null) {
    parts.push(escapeHtml(source.slice(lastIndex, match.index)));

    const rawUrl = match[1];
    let href = rawUrl;
    let downloadName = '';

    try {
      const url = new URL(rawUrl, window.location.origin);
      if (url.pathname.indexOf('/api/v1/download/') !== -1) {
        url.search = '';
        url.hash = '';
        href = url.href;
        const segment = url.pathname.split('/').pop() || '';
        const named = /^([a-f0-9]{64})_(.+)$/i.exec(segment);
        downloadName = named ? named[2] : segment;
      }
    } catch (e) {
      // 保留原始链接
    }

    const downloadAttr = downloadName
      ? ' download="' + escapeHtml(downloadName) + '"'
      : '';
    parts.push(
      '<a class="msg-link" href="' +
        escapeHtml(href) +
        '"' +
        downloadAttr +
        '>' +
        escapeHtml(rawUrl) +
        '</a>',
    );
    lastIndex = match.index + match[0].length;
  }

  parts.push(escapeHtml(source.slice(lastIndex)));
  return parts.join('');
}

export function formatMessageTime(value) {
  if (!value) return '';
  try {
    return new Date(value).toLocaleString();
  } catch (e) {
    return '';
  }
}
