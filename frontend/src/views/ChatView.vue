<template>
  <div class="chat-shell" :class="{ 'has-current': current, 'details-open': showDetails }">
    <aside class="rail">
      <AppLogo compact />
      <button class="rail-btn active"><MessageSquareText :size="19" /></button>
      <RouterLink v-if="auth.isAdmin" to="/admin/users" class="rail-btn"><ShieldCheck :size="19" /></RouterLink>
      <div class="grow"></div>
      <button class="rail-btn" @click="logout"><LogOut :size="19" /></button>
    </aside>

    <section class="rooms">
      <header>
        <div><b>消息</b><span>{{ rooms.length }} 个会话</span></div>
        <button class="square" @click="showCreate = !showCreate"><Plus :size="17" /></button>
      </header>

      <div v-if="showCreate" class="new-room">
        <input v-model="newRoom" class="input" placeholder="群聊名称" @keyup.enter="createRoom" />
        <button class="btn btn-primary btn-sm" :disabled="creatingRoom" @click="createRoom">
          {{ creatingRoom ? '创建中' : '创建' }}
        </button>
      </div>

      <div v-if="showCreate" class="join-room">
        <input v-model="joinRoomId" class="input" placeholder="输入群号加入" @keyup.enter="joinRoom" />
        <button class="btn btn-primary btn-sm" :disabled="joiningRoom" @click="joinRoom">
          {{ joiningRoom ? '加入中' : '加入' }}
        </button>
      </div>

      <div class="room-search"><Search :size="14" /><input v-model="keyword" placeholder="搜索会话" /></div>

      <div class="room-list"><div v-if="!filteredRooms.length" class="room-empty">{{ keyword ? '没有匹配的会话' : '点击右上角 + 创建或加入群聊' }}</div>
        <button
          v-for="r in filteredRooms"
          :key="r.id"
          class="room"
          :class="{ active: current?.id === r.id }"
          @click="selectRoom(r)"
          @contextmenu.prevent="openRoomMenu($event, r)"
        >
          <div class="room-avatar">{{ String(r.name || r.room_name || '群').slice(0, 1) }}</div>
          <div class="room-meta">
            <b>{{ r.name || r.room_name || `群聊 #${r.id}` }}</b>
            <span>{{ r.last_message || '点击查看聊天记录' }}</span>
          </div>
          <span class="time">{{ r.time || '' }}</span>
        </button>
      </div>
    </section>

    <main class="conversation">
      <header>
        <button v-if="current" class="square back-button" aria-label="返回会话列表" @click="current = null"><ArrowLeft :size="18" /></button><div class="header-title">
          <h2>{{ current?.name || current?.room_name || '选择一个会话' }}</h2>
          <span v-if="current">Room #{{ current.id }}</span>
        </div>
        <div class="header-actions">
          <button v-if="current" class="square" aria-label="搜索聊天记录" @click="showHistory = !showHistory"><Search :size="17" /></button>
          <button v-if="current" class="square" aria-label="会话信息与 Agent 管理" @click="showDetails = !showDetails"><Users :size="17" /></button>
          <div class="conn"><span :class="socketState"></span>{{ socketState === 'online' ? '实时连接' : '离线' }}</div>
          <div class="user-chip" :title="auth.user?.username || ''">
            <img v-if="auth.user?.avatar" :src="auth.user.avatar" alt="" />
            <span v-else>{{ String(auth.user?.username || 'U').slice(0, 1).toUpperCase() }}</span>
          </div>
        </div>
      </header>

      <div v-if="current && showHistory" class="history-search">
        <Search :size="14" />
        <input v-model="historyKeyword" placeholder="按关键字搜索聊天记录" @keyup.enter="searchHistory" />
        <button class="btn btn-sm" :disabled="historySearching" @click="searchHistory">
          {{ historySearching ? '搜索中' : '搜索' }}
        </button>
        <button v-if="historySearchActive" class="btn btn-sm" @click="clearHistorySearch">返回聊天</button>
      </div>

      <div ref="msgBox" class="messages">
        <div v-if="!current" class="empty">
          <MessagesSquare :size="36" />
          <b>选择左侧群聊开始交流</b>
          <span>消息、文件与 @Agent 交互都会显示在这里</span>
        </div>

        <div v-if="historySearchActive" class="search-summary">找到 {{ searchResults.length }} 条相关消息</div>

        <div v-if="historySearchActive && !searchResults.length" class="empty">没有找到相关消息</div>

        <div
          v-for="m in displayedMessages"
          :key="messageKey(m)"
          class="message"
          :class="{ mine: isMine(m) }"
        >
          <div class="msg-avatar">
            <img v-if="avatarOf(m)" :src="avatarOf(m)" alt="" />
            <span v-else>{{ initialOf(m) }}</span>
          </div>
          <div class="bubble">
            <div class="sender">{{ displayName(m) }}</div>
                        <div class="message-content">
              <template v-for="(part, partIndex) in messageParts(m)" :key="partIndex">
                <a v-if="part.type === 'link'" :href="part.url" class="file-link" @click.prevent="downloadMessageFile(part)">{{ part.text || '下载文件' }}</a>
                <span v-else>{{ part.text }}</span>
              </template>
            </div>
            <time>{{ formatTime(m.created_at || m.timestamp) }}</time>
          </div>
        </div>
      </div>

      <div v-if="uploading" class="upload-progress">
        <div><Paperclip :size="13" /><span>{{ uploadStageText }}</span></div>
        <b>{{ uploadProgress }}%</b>
        <i><em :style="{ width: uploadProgress + '%' }"></em></i>
      </div>

      <footer>
        <input ref="fileInput" type="file" hidden @change="handleFile" />
        <div class="emoji-wrap">
          <button class="attach" type="button" :disabled="uploading || !current" @click="showEmoji = !showEmoji">
            <Smile :size="17" />
          </button>
          <div v-if="showEmoji && current" class="emoji-panel">
            <button v-for="e in emojis" :key="e" type="button" @click="insertEmoji(e)">{{ e }}</button>
          </div>
        </div>
        <button class="attach" :disabled="uploading || !current" @click="fileInput?.click()"><Paperclip :size="17" /></button>
        <textarea
          v-model="draft"
          class="composer"
          rows="1"
          :disabled="!current"
          :placeholder="current ? '输入消息，使用 @agent 与智能助手对话' : '请先选择左侧群聊'"
          @keydown.enter.exact.prevent="send"
        ></textarea>
        <button class="send" :disabled="!current || uploading" @click="send"><Send :size="17" /></button>
      </footer>
    </main>

    <aside class="detail">
      <header>会话信息<button class="square" aria-label="关闭会话信息" @click="showDetails = false"><X :size="17" /></button></header>
      <div v-if="current" class="detail-body">
        <div class="room-big">{{ String(current.name || current.room_name || '群').slice(0, 1) }}</div>
        <h3>{{ current.name || current.room_name }}</h3>
        <p>Room #{{ current.id }}</p>
        <section class="agent-card">
<div class="agent-heading"><Bot :size="20" /><b>群聊 Agent</b><span>{{ agentEnabled ? '已启用' : '已停用' }}</span></div>
<p>在聊天中使用 @agent 与助手对话。</p>
<div v-if="canManageAgent" class="agent-actions"><button class="btn" :disabled="agentLoading" @click="toggleAgent">{{ agentLoading ? '处理中…' : agentEnabled ? '停用 Agent' : '启用 Agent' }}</button><button class="btn danger" :disabled="agentLoading" @click="removeAgent"><Trash2 :size="14" />移除 Agent 账号</button></div>
<p v-else>仅群主和管理员可以管理 Agent。</p><p v-if="agentMessage" role="status">{{ agentMessage }}</p>
</section><div class="info-row"><Users :size="15" /><span>成员</span><b>{{ members.length }}</b></div>
        <div class="members">
          <div
            v-for="m in members"
            :key="m.id || m.user_id"
            class="member-row"
            @contextmenu.prevent="openMemberMenu($event, m)"
          >
            <img v-if="m.avatar" class="member-avatar" :src="m.avatar" alt="" />
            <span v-else>{{ initialOf(m) }}</span>
            {{ displayName(m) }}
          </div>
        </div>
      </div>
    </aside>
  </div>

  <div
    v-if="contextMenu.visible"
    class="context-menu"
    :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
    @click.stop
  >
    <button v-if="contextMenu.member" @click="kickMember">踢出群聊</button>
    <button v-else @click="leaveRoom">退出群聊</button>
    <button @click="disbandRoom">解散群聊</button>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  MessageSquareText,
  ShieldCheck,
  LogOut,
  Plus,
  Search,
  MessagesSquare,
  Send,
  Users,
  Paperclip,
  Smile,
  Bot,
  ArrowLeft,
  X,
  Trash2,
} from 'lucide-vue-next'
import AppLogo from '../components/common/AppLogo.vue'
import { useAuthStore } from '../stores/auth.js'
import { imApi, createImSocket, normalizeSocketMessage } from '../api/im.js'
import { mergeMessages, messageKey } from '../utils/message-order.js'
import { uploadFile } from '../services/uploader.js'

const auth = useAuthStore()
const router = useRouter()
const rooms = ref([])
const current = ref(null)
const messages = ref([])
const members = ref([])
const keyword = ref('')
const draft = ref('')
const newRoom = ref('')
const showCreate = ref(false)
const showEmoji = ref(false)
const socketState = ref('offline')
const showDetails = ref(false)
const showHistory = ref(false)
const agentMessage = ref('')
const canManageAgent = computed(() => auth.isSuperAdmin || Number(current.value?.my_role) >= 2 || String(current.value?.creator_id) === String(auth.user?.id ?? auth.user?.user_id))
const agentEnabled = ref(false)
const agentLoading = ref(false)
const msgBox = ref()
const fileInput = ref()
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadStageText = ref('准备上传')
const creatingRoom = ref(false)
const joinRoomId = ref('')
const joiningRoom = ref(false)
const historyKeyword = ref('')
const historySearching = ref(false)
const historySearchActive = ref(false)
const searchResults = ref([])
const contextMenu = ref({ visible: false, x: 0, y: 0, room: null, member: null })
let ws

const emojis = ['😀', '😁', '😂', '🤣', '😊', '😍', '😘', '😎', '🤔', '🙄', '😭', '😡', '👍', '👎', '👏', '🙏', '💪', '🤝', '🎉', '❤️', '🔥', '⭐', '✅', '❌', '💯']

const memberMap = computed(() => {
  const map = new Map()
  members.value.forEach((m) => map.set(String(m.id ?? m.user_id), m))
  return map
})

const filteredRooms = computed(() =>
  rooms.value.filter((r) => String(r.name || r.room_name || '').toLowerCase().includes(keyword.value.toLowerCase())),
)

const displayedMessages = computed(() => (historySearchActive.value ? searchResults.value : messages.value))

function normalizeList(data) {
  return Array.isArray(data) ? data : (data?.items || data?.rooms || data?.members || data?.messages || data?.data || [])
}

function isMine(m) {
  return Number(m.user_id || m.sender_id) === Number(auth.user?.id || auth.user?.user_id)
}

function displayName(m) {
  return m.username || m.sender_name || m.user_name || m.name || (isMine(m) ? (auth.user?.username || '我') : `User ${m.user_id || m.sender_id || ''}`)
}

function avatarOf(m) {
  if (m.avatar) return m.avatar
  const member = memberMap.value.get(String(m.user_id || m.sender_id || m.id))
  if (member?.avatar) return member.avatar
  if (isMine(m)) return auth.user?.avatar || ''
  return ''
}

function initialOf(m) {
  const name = displayName(m)
  return String(name || 'U').slice(0, 1).toUpperCase()
}

function extractFileReference(content) {
  const text = String(content || '')
  const typed = text.match(/^\[文件\]\s+(.+?)\s+(\/api\/v1\/(?:files\/\d+\/download|download\/[^\s]+))$/)
  if (typed) return { name: typed[1], url: typed[2] }
  const fallback = text.match(/(\/api\/v1\/(?:files\/\d+\/download|download\/[^\s]+))/)
  return fallback ? { name: '下载文件', url: fallback[1] } : null
}

function messageParts(m) {
  const text = String(m.content || '')
  const file = extractFileReference(text)
  if (!file) return [{ type: 'text', text }]
  const idx = text.indexOf(file.url)
  return [
    { type: 'text', text: file.name === '下载文件' ? text.slice(0, idx) : '' },
    { type: 'link', text: file.name, url: file.url },
    { type: 'text', text: text.slice(idx + file.url.length) },
  ]
}

async function downloadMessageFile(part) {
  try {
    const authorizeRes = await fetch(part.url, {
      headers: auth.token ? { Authorization: `Bearer ${auth.token}` } : {},
    })
    const authorizeData = await authorizeRes.json().catch(() => null)
    if (!authorizeRes.ok) {
      throw new Error(authorizeData?.error || `下载授权失败 (${authorizeRes.status})`)
    }
    if (!authorizeData?.download_url) throw new Error('下载授权响应缺少文件地址')
    // 对象存储的预签名 URL 已携带自身认证信息，这个请求不能再附加 JWT。
    const downloadRes = await fetch(authorizeData.download_url)
    if (!downloadRes.ok) throw new Error(`文件下载失败 (${downloadRes.status})`)
    const blobUrl = URL.createObjectURL(await downloadRes.blob())
    const anchor = document.createElement('a')
    anchor.href = blobUrl
    anchor.download = part.text || '下载文件'
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(blobUrl)
  } catch (e) {
    alert(e.message || '文件下载失败')
  }
}

function enrichMessage(m) {
  const member = memberMap.value.get(String(m.user_id || m.sender_id || m.id))
  return {
    ...m,
    username: m.username || member?.username || member?.name || '',
    avatar: m.avatar || member?.avatar || (isMine(m) ? auth.user?.avatar || '' : ''),
  }
}

function scrollToBottom() {
  nextTick(() => msgBox.value?.scrollTo({ top: msgBox.value.scrollHeight, behavior: 'smooth' }))
}

function insertEmoji(emoji) {
  draft.value += emoji
  showEmoji.value = false
  nextTick(() => {
    const el = document.querySelector('.composer')
    el?.focus()
  })
}

async function loadRooms() {
  try {
    rooms.value = normalizeList(await imApi.myRooms())
  } catch {
    rooms.value = []
  }
}

async function selectRoom(r) {
  current.value = r
  showEmoji.value = false
  agentEnabled.value = Boolean(r.agent_enabled)
  agentMessage.value = ''
  historyKeyword.value = ''
  historySearching.value = false
  historySearchActive.value = false
  searchResults.value = []
  messages.value = []
  members.value = []

  const [msgRes, memberRes] = await Promise.allSettled([imApi.messages(r.id), imApi.members(r.id)])
  if (current.value?.id !== r.id) return
  members.value = memberRes.status === 'fulfilled' ? normalizeList(memberRes.value) : []
  // 历史请求期间可能已经收到实时消息，合并而不是覆盖，避免丢气泡或重复展示。
  messages.value = mergeMessages((msgRes.status === 'fulfilled' ? normalizeList(msgRes.value) : []).map(enrichMessage), messages.value)
  scrollToBottom()
}

async function toggleAgent() { await changeAgent(false) }
async function removeAgent() {
  if (!current.value || agentLoading.value) return
  if (!confirm('移除该群的 Agent 账号及配置？历史消息保留，其他群不受影响。')) return
  await changeAgent(true)
}
async function changeAgent(remove) {
  if (!current.value || agentLoading.value || !canManageAgent.value) return
  const room = current.value
  const enabled = !room.agent_enabled
  agentLoading.value = true
  agentMessage.value = ''
  try {
    if (remove) await imApi.removeAgent(room.id)
    else if (enabled) await imApi.enableAgent(room.id)
    else await imApi.disableAgent(room.id)
    room.agent_enabled = remove ? false : enabled
    if (current.value?.id === room.id) {
      agentEnabled.value = room.agent_enabled
      agentMessage.value = remove ? '已移除该群的 Agent 账号与配置' : enabled ? 'Agent 已启用' : 'Agent 已停用'
      try {
        const fresh = await imApi.members(room.id)
        if (current.value?.id === room.id) members.value = fresh
      } catch { agentMessage.value += '；成员列表刷新失败，请重新打开会话' }
    }
  } catch (e) {
    if (current.value?.id === room.id) agentMessage.value = e.message || '操作失败，请重试'
  } finally { agentLoading.value = false }
}

async function createRoom() {
  const name = newRoom.value.trim()
  if (!name || creatingRoom.value) return
  creatingRoom.value = true
  try {
    const room = await imApi.createRoom({ name })
    newRoom.value = ''
    showCreate.value = false
    if (room?.id) {
      const exists = rooms.value.some((r) => String(r.id) === String(room.id))
      if (!exists) rooms.value.unshift(room)
      await selectRoom(room)
    } else {
      await loadRooms()
    }
  } catch (e) {
    alert(e.message || '创建群聊失败')
  } finally {
    creatingRoom.value = false
  }
}

async function joinRoom() {
  const roomId = joinRoomId.value.trim()
  if (!roomId || joiningRoom.value) return
  joiningRoom.value = true
  try {
    await imApi.joinRoom(roomId)
    joinRoomId.value = ''
    await loadRooms()
    const joined = rooms.value.find((r) => String(r.id) === String(roomId))
    if (joined) await selectRoom(joined)
  } catch (e) {
    alert(e.message || '加入群聊失败')
  } finally {
    joiningRoom.value = false
  }
}

async function searchHistory() {
  const keyword = historyKeyword.value.trim()
  if (!keyword || !current.value || historySearching.value) return

  historySearching.value = true
  try {
    const hits = normalizeList(await imApi.searchMessages(current.value.id, { q: keyword }))
    searchResults.value = hits.map(enrichMessage)
    historySearchActive.value = true
  } catch (e) {
    alert(e.message || '历史消息搜索失败')
  } finally {
    historySearching.value = false
  }
}

function clearHistorySearch() {
  historyKeyword.value = ''
  historySearching.value = false
  historySearchActive.value = false
  searchResults.value = []
}

function connect() {
  try {
    ws = createImSocket(auth.token)
    ws.onopen = () => (socketState.value = 'online')
    ws.onclose = () => (socketState.value = 'offline')
    ws.onerror = () => (socketState.value = 'offline')
    ws.onmessage = (ev) => {
      try {
        const raw = normalizeSocketMessage(JSON.parse(ev.data))
        if (current.value && (!raw.room_id || Number(raw.room_id) === Number(current.value.id))) {
          const m = enrichMessage(raw)
          messages.value = mergeMessages(messages.value, [m])
          scrollToBottom()
        }
      } catch {}
    }
  } catch {
    socketState.value = 'offline'
  }
}

function send() {
  const content = draft.value.trim()
  if (!content || !current.value || ws?.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ room_id: current.value.id, content, client_msg_id: globalThis.crypto?.randomUUID?.() || String(Date.now()) }))
  draft.value = ''
}

async function handleFile(ev) {
  const file = ev.target.files?.[0]
  if (!file || !current.value) return
  uploading.value = true
  uploadProgress.value = 0
  uploadStageText.value = '计算 SHA-256'
  try {
    const result = await uploadFile(file, {
      roomId: current.value.id,
      onStage: (s) => (uploadStageText.value = ({ hash: '计算 SHA-256', presign: '获取上传链接', upload: '上传文件', complete: '登记文件' }[s] || s)),
      onProgress: (p) => (uploadProgress.value = p),
    })
    if (ws?.readyState !== WebSocket.OPEN) throw new Error('文件已上传，但 WebSocket 已断开，无法发送文件消息')
    ws.send(JSON.stringify({ room_id: current.value.id, content: `[文件] ${result.file_name} ${result.download_url}`, client_msg_id: globalThis.crypto?.randomUUID?.() || String(Date.now()) }))
  } catch (e) {
    alert(e.message || '文件上传失败')
  } finally {
    uploading.value = false
    uploadProgress.value = 0
    ev.target.value = ''
  }
}

function formatTime(v) {
  if (!v) return ''
  const d = new Date(v)
  return Number.isNaN(d.valueOf()) ? '' : d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function logout() {
  ws?.close()
  auth.logout()
  router.replace('/login')
}

function openRoomMenu(event, room) {
  event.preventDefault()
  contextMenu.value = { visible: true, x: event.clientX, y: event.clientY, room, member: null }
}

function openMemberMenu(event, member) {
  event.preventDefault()
  if (!current.value) return
  contextMenu.value = { visible: true, x: event.clientX, y: event.clientY, room: current.value, member }
}

function closeContextMenu() {
  contextMenu.value.visible = false
}

function memberUserId(member) {
  return member.user_id ?? member.id
}

async function kickMember() {
  const { room, member } = contextMenu.value
  if (!room || !member) return closeContextMenu()
  const userId = memberUserId(member)
  if (!confirm(`确定将 ${displayName(member)} 踢出群聊？`)) return
  try {
    await imApi.removeMember(room.id, userId)
    members.value = members.value.filter((m) => String(memberUserId(m)) !== String(userId))
    alert('已踢出群聊')
  } catch (e) {
    alert(e.message || '踢出失败')
  } finally {
    closeContextMenu()
  }
}

async function leaveRoom() {
  const { room } = contextMenu.value
  if (!room) return closeContextMenu()
  const userId = auth.user?.id ?? auth.user?.user_id
  if (!confirm(`确定退出群聊「${room.name || room.room_name || room.id}」？`)) return
  try {
    await imApi.removeMember(room.id, userId)
    rooms.value = rooms.value.filter((r) => String(r.id) !== String(room.id))
    if (current.value && String(current.value.id) === String(room.id)) {
      current.value = null
      messages.value = []
      members.value = []
    }
    alert('已退出群聊')
  } catch (e) {
    alert(e.message || '退出失败')
  } finally {
    closeContextMenu()
  }
}

async function disbandRoom() {
  const { room } = contextMenu.value
  if (!room) return closeContextMenu()
  if (!confirm(`确定解散群聊「${room.name || room.room_name || room.id}」？`)) return
  try {
    await imApi.disbandRoom(room.id)
    rooms.value = rooms.value.filter((r) => String(r.id) !== String(room.id))
    if (current.value && String(current.value.id) === String(room.id)) {
      current.value = null
      messages.value = []
      members.value = []
    }
    alert('群聊已解散')
  } catch (e) {
    alert(e.message || '解散失败')
  } finally {
    closeContextMenu()
  }
}

onMounted(() => {
  loadRooms()
  connect()
  window.addEventListener('click', closeContextMenu)
})
onBeforeUnmount(() => {
  ws?.close()
  window.removeEventListener('click', closeContextMenu)
})
</script>

<style scoped>
.chat-shell {
  height: 100vh;
  display: grid;
  grid-template-rows: minmax(0, 1fr); height: 100dvh;
  grid-template-columns: 68px 300px minmax(0, 1fr);
  background: var(--surface);
  overflow: hidden;
}

.chat-shell > * {
  min-width: 0;
  min-height: 0;
}

.rail,
.rooms,
.conversation,
.detail {
  height: 100%;
  min-height: 0;
}

.rail {
  border-right: 1px solid var(--line);
  padding: 15px 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 9px;
  flex-shrink: 0;
}

.rail .grow {
  flex: 1;
  min-height: 0;
}

.rail-btn,
.square {
  width: 38px;
  height: 38px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--text-2);
  display: grid;
  place-items: center;
  flex-shrink: 0;
}

.rail-btn:hover,
.rail-btn.active {
  background: var(--primary-soft);
  color: var(--primary);
}

.rooms {
  border-right: 1px solid var(--line);
  background: var(--surface-soft);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.rooms > header,
.conversation > header,
.detail > header {
  height: 68px;
  padding: 0 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--line);
  background: var(--surface);
  flex-shrink: 0;
}

.rooms header b {
  display: block;
  font-size: 16px;
}

.rooms header span {
  display: block;
  font-size: 10px;
  color: var(--text-3);
  margin-top: 3px;
}

.square {
  background: var(--surface);
  border: 1px solid var(--line);
}

.new-room,
.join-room {
  padding: 10px;
  display: flex;
  gap: 6px;
}

.history-search {
  margin: 0 16px 10px;
  padding: 8px 10px;
  border: 1px solid var(--line-strong);
  border-radius: 10px;
  background: var(--surface-soft);
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--text-3);
  flex-shrink: 0;
}

.history-search input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--text);
}

.history-search .btn {
  flex-shrink: 0;
}

.search-summary {
  margin: 0 0 8px;
  font-size: 11px;
  color: var(--text-2);
}

.room-search {
  margin: 0 12px 12px;
  height: 36px;
  border: 1px solid var(--line);
  border-radius: 9px;
  background: var(--surface);
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 10px;
  color: var(--text-3);
  flex-shrink: 0;
}

.room-search input {
  border: 0;
  outline: 0;
  background: transparent;
  width: 100%;
  color: var(--text);
}

.room-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 0 8px 12px;
}

.room {
  width: 100%;
  border: 0;
  background: transparent;
  padding: 10px;
  border-radius: 10px;
  display: grid;
  grid-template-columns: 38px 1fr auto;
  gap: 10px;
  text-align: left;
  margin: 2px 0;
  color: var(--text);
}

.room:hover,
.room.active {
  background: var(--surface);
}

.room-avatar {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  background: linear-gradient(145deg, #dce4ff, #eef2ff);
  color: var(--primary);
  display: grid;
  place-items: center;
  font-weight: 800;
}

.room-meta {
  min-width: 0;
}

.room-meta b,
.room-meta span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.room-meta b {
  font-size: 12px;
  margin: 2px 0 5px;
}

.room-meta span,
.time {
  font-size: 10px;
  color: var(--text-3);
}

.conversation {
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  overflow: hidden;
}

.conversation h2 {
  font-size: 14px;
  margin: 0;
}

.conversation header .header-title {
  min-width: 0;
}

.conversation header .header-title h2,
.conversation header .header-title span {
  display: block;
}

.conversation header .header-title span {
  font-size: 10px;
  color: var(--text-3);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 9px;
}

.agent-toggle {
  display: flex;
  align-items: center;
  gap: 5px;
  white-space: nowrap;
}

.conn {
  font-size: 11px;
  color: var(--text-3);
  display: flex;
  align-items: center;
  gap: 6px;
}

.conn span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--text-3);
}

.conn .online {
  background: var(--success);
}

.user-chip {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  overflow: hidden;
  display: grid;
  place-items: center;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 12px;
  font-weight: 800;
  flex-shrink: 0;
}

.user-chip img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 22px;
  background: linear-gradient(180deg, var(--surface) 0%, var(--surface-soft) 100%);
  overscroll-behavior: contain;
  scroll-behavior: smooth;
}

.empty {
  height: 100%;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 9px;
  color: var(--text-3);
}

.empty b {
  color: var(--text-2);
}

.message {
  display: flex;
  margin: 10px 0;
  align-items: flex-start;
}

.message.mine {
  justify-content: flex-end;
}

.msg-avatar {
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  border-radius: 9px;
  background: var(--surface-muted);
  color: var(--text-2);
  display: grid;
  place-items: center;
  font-size: 11px;
  font-weight: 800;
  overflow: hidden;
}

.msg-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.message:not(.mine) .msg-avatar {
  margin-right: 8px;
}

.message.mine .msg-avatar {
  order: 2;
  margin-left: 8px;
}

.message-content {
  white-space: pre-wrap;
  word-break: break-word;
}

.file-link {
  color: var(--primary);
  text-decoration: underline;
  cursor: pointer;
}

.bubble {
  max-width: min(620px, 78%);
  padding: 9px 12px;
  border: 1px solid #dbe4ff;
  background: var(--primary-soft);
  border-radius: 4px 13px 13px 13px;
  line-height: 1.55;
  color: var(--text);
  box-shadow: var(--shadow-sm);
}

.message.mine .bubble {
  background: #dce7ff;
  border-color: #c9d7ff;
  border-radius: 13px 4px 13px 13px;
}

.sender {
  font-size: 10px;
  color: var(--text-3);
  margin-bottom: 3px;
}

.message.mine .sender,
.message.mine .bubble time {
  color: var(--primary);
}

.bubble time {
  display: block;
  font-size: 9px;
  color: var(--text-3);
  margin-top: 4px;
  text-align: right;
}

.context-menu {
  position: fixed;
  z-index: 1000;
  min-width: 132px;
  padding: 6px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 10px;
  box-shadow: var(--shadow-lg);
  display: grid;
  gap: 2px;
}

.context-menu button {
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--text);
  padding: 8px 10px;
  border-radius: 7px;
  cursor: pointer;
  font-size: 12px;
  text-align: left;
}

.context-menu button:hover {
  background: var(--surface-soft);
  color: var(--primary);
}

.upload-progress {
  margin: 0 16px 6px;
  padding: 9px 11px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface-soft);
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 5px 10px;
  align-items: center;
  font-size: 10px;
  color: var(--text-2);
  flex-shrink: 0;
}

.upload-progress > div {
  display: flex;
  align-items: center;
  gap: 6px;
}

.upload-progress > b {
  color: var(--primary);
}

.upload-progress > i {
  grid-column: 1 / -1;
  height: 4px;
  background: var(--surface-muted);
  border-radius: 99px;
  overflow: hidden;
}

.upload-progress em {
  display: block;
  height: 100%;
  background: var(--primary);
  border-radius: 99px;
}

.conversation footer {
  padding: 12px 16px;
  border-top: 1px solid var(--line);
  display: flex;
  gap: 8px;
  flex-shrink: 0;
  position: relative;
}

.attach {
  width: 42px;
  border: 1px solid var(--line-strong);
  border-radius: 11px;
  background: var(--surface);
  color: var(--text-2);
  display: grid;
  place-items: center;
  flex-shrink: 0;
}

.attach:hover {
  color: var(--primary);
  border-color: var(--primary);
}

.emoji-wrap {
  position: relative;
  flex-shrink: 0;
}

.emoji-panel {
  position: absolute;
  bottom: 48px;
  left: 0;
  width: 288px;
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 4px;
  padding: 8px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 12px;
  box-shadow: var(--shadow-lg);
  z-index: 20;
}

.emoji-panel button {
  height: 30px;
  border: 0;
  background: transparent;
  border-radius: 7px;
  font-size: 16px;
  cursor: pointer;
}

.emoji-panel button:hover {
  background: var(--surface-soft);
}

.composer {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--line-strong);
  border-radius: 11px;
  padding: 10px 12px;
  outline: 0;
  resize: none;
  background: var(--surface-soft);
  color: var(--text);
}

.send {
  width: 42px;
  border: 0;
  border-radius: 11px;
  background: var(--primary);
  color: #fff;
  flex-shrink: 0;
}

.detail {
  border-left: 1px solid var(--line);
  background: var(--surface);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.detail > header {
  font-weight: 700;
  font-size: 12px;
}

.detail-body {
  text-align: center;
  padding: 24px 18px;
  overflow-y: auto;
}

.room-big {
  width: 58px;
  height: 58px;
  margin: auto;
  border-radius: 16px;
  background: var(--primary-soft);
  color: var(--primary);
  display: grid;
  place-items: center;
  font-size: 20px;
  font-weight: 800;
}

.detail h3 {
  margin: 12px 0 2px;
  font-size: 14px;
}

.detail p {
  margin: 0;
  color: var(--text-3);
  font-size: 10px;
}

.info-row {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 8px;
  align-items: center;
  margin-top: 26px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
  text-align: left;
  color: var(--text-2);
  font-size: 11px;
}

.members {
  margin-top: 10px;
}

.members div {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  padding: 6px 0;
  text-align: left;
}

.members span {
  width: 24px;
  height: 24px;
  border-radius: 7px;
  background: var(--surface-muted);
  display: grid;
  place-items: center;
}

.member-avatar {
  width: 24px;
  height: 24px;
  border-radius: 7px;
  object-fit: cover;
}

@media (max-width: 1100px) {
  .chat-shell {
    grid-template-columns: 68px 260px minmax(0, 1fr);
  }

  .detail {
    display: none;
  }
}

@media (max-width: 720px) {
  .chat-shell {
    grid-template-columns: 64px minmax(0, 1fr);
  }

  .rooms {
    display: none;
  }
}

.detail { display: none; position: fixed; inset: 0 0 0 auto; width: min(340px, 100vw); z-index: 40; box-shadow: -12px 0 40px #17203320; }
.details-open .detail { display: flex; }
.back-button { display: none; }
.rooms > header, .conversation > header { height: 80px; gap: 16px; padding: 0 24px; }
.header-title { flex: 1; }
.conversation h2 { font-size: 17px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.header-title span { margin-top: 5px; }
.room-search { margin: 20px 16px 12px; height: 42px; }
.room { padding: 14px 12px; margin-bottom: 6px; }
.room.active { background: var(--primary-soft); box-shadow: inset 3px 0 var(--primary); }
.room-meta b { font-size: 14px; }
.room-meta span, .rooms header span { font-size: 12px; }
.room-empty { padding: 28px 16px; color: var(--text-3); line-height: 1.8; text-align: center; }
.new-room, .join-room { padding: 12px 16px 0; gap: 8px; }
.new-room .input, .join-room .input { min-width: 0; flex: 1; }
.history-search { margin: 16px 24px 0; }
.messages { padding: 28px clamp(20px, 5vw, 80px); }
.message { margin: 0 0 24px; }
.bubble { padding: 12px 16px; font-size: 14px; line-height: 1.75; background: var(--surface); border-color: var(--line); }
.sender { font-size: 12px; margin-bottom: 6px; }
.bubble time { font-size: 10px; }
.conversation footer { padding: 20px 24px; align-items: center; gap: 10px; }
.composer { min-height: 66px; padding: 14px; line-height: 1.6; }
.attach, .send { height: 42px; }
.agent-card { margin-top: 28px; padding: 18px; border: 1px solid var(--line); border-radius: 14px; background: var(--surface-soft); text-align: left; }
.agent-heading { display: flex; align-items: center; gap: 8px; font-size: 14px; }
.agent-heading span { margin-left: auto; font-size: 11px; color: var(--text-2); }
.agent-card p { font-size: 12px; line-height: 1.8; margin-top: 12px; }
.agent-actions { display: grid; gap: 10px; margin-top: 16px; }
.danger { color: var(--danger); }
.members div { padding: 10px 0; font-size: 13px; overflow-wrap: anywhere; }
.members span, .member-avatar { flex-shrink: 0; }
.detail h3 { overflow-wrap: anywhere; }
button:focus-visible, a:focus-visible { outline: 2px solid var(--primary); outline-offset: 3px; }
@media (max-width: 900px) {
.chat-shell { grid-template-columns: 56px 240px minmax(0, 1fr); }
.conn { display: none; }
.conversation > header { padding: 0 16px; }
.conversation footer { padding: 16px; gap: 6px; }
}
@media (max-width: 720px) {
.chat-shell { grid-template-columns: 52px minmax(0, 1fr); }
.rooms { display: flex; }
.conversation { display: none; }
.has-current .rooms { display: none; }
.has-current .conversation { display: flex; }
.back-button { display: grid; }
.conversation > header { gap: 8px; height: 72px; }
.header-actions { gap: 6px; }
.user-chip { display: none; }
.messages { padding: 20px 14px; }
.conversation footer { flex-wrap: wrap; padding: 12px; }
.composer { order: -1; flex-basis: 100%; }
.send { margin-left: auto; }
.history-search { margin: 12px; flex-wrap: wrap; }
.empty { text-align: center; line-height: 1.7; }
}
</style>
