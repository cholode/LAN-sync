// 保留 64 位整数的字符串精度，禁止先转换成 Number 再排序。
function integer(value) {
  try { return BigInt(value || 0) } catch { return 0n }
}

function keys(message) {
  const room = String(message.room_id ?? '')
  const result = []
  if (integer(message.id) > 0n) result.push(`id:${room}:${message.id}`)
  if (integer(message.room_seq) > 0n) result.push(`seq:${room}:${message.room_seq}`)
  if (message.client_msg_id) result.push(`client:${room}:${message.sender_id ?? message.user_id}:${message.client_msg_id}`)
  return result
}

export function messageKey(message) {
  return keys(message)[0] || message.id
}

// 同时合并历史、实时与重放消息；不能把晚到的小序号消息直接丢弃。
export function mergeMessages(...batches) {
  const result = []
  const positions = new Map()
  for (const message of batches.flat()) {
    const aliases = keys(message)
    const found = aliases.map(key => positions.get(key)).find(index => index !== undefined)
    const index = found ?? result.length
    if (found === undefined) result.push(message)
    else result[index] = { ...message, ...result[index] }
    for (const key of aliases) positions.set(key, index)
  }
  return result.sort((a, b) => {
    const left = integer(a.room_seq), right = integer(b.room_seq)
    if (left !== right) return left < right ? -1 : 1
    const leftID = integer(a.id), rightID = integer(b.id)
    if (leftID !== rightID) return leftID < rightID ? -1 : 1
    return String(a.created_at || '').localeCompare(String(b.created_at || ''))
  })
}
