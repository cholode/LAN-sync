import test from 'node:test'
import assert from 'node:assert/strict'
import { mergeMessages } from './message-order.js'

test('晚到消息按群序号补入，重复广播和历史只显示一次', () => {
  const a = { id: '9007199254740993', room_seq: '43', room_id: '1', sender_id: '2', client_msg_id: 'C1' }
  const b = { id: '9007199254740994', room_seq: '44', room_id: '1', sender_id: '3', client_msg_id: 'C1' }
  const result = mergeMessages([b], [a, b], [a])
  assert.deepEqual(result, [a, b])
})

test('大整数序号保留精度，同一群序号重复不会增加气泡', () => {
  const a = { id: '10', room_id: '1', room_seq: '9007199254740992' }
  const b = { id: '9', room_id: '1', room_seq: '9007199254740993' }
  assert.deepEqual(mergeMessages([b, a, { ...a }]), [a, b])
})

test('历史和实时使用发送者作用域，不误合并不同用户的凭证', () => {
  const a = { room_id: '1', sender_id: '2', client_msg_id: 'C1' }
  const b = { room_id: '1', sender_id: '3', client_msg_id: 'C1' }
  assert.equal(mergeMessages([a,b,a]).length, 2)
})
