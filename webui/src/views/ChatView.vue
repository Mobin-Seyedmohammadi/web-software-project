<template>
  <div class="chat-view">
    <!-- Header -->
    <div class="chat-header">
      <button @click="goBack" class="btn btn-icon">&#8592; Back</button>
      <div v-if="conversation" class="chat-title" @click="isGroup ? (showGroupPanel = !showGroupPanel) : null"
           :class="{ clickable: isGroup }">
        <div class="avatar avatar-sm" :style="headerAvatarStyle">
          <img v-if="conversation.displayPhotoUrl" :src="resolveUrl(conversation.displayPhotoUrl)" class="avatar-img" />
          <span v-else>{{ initial(conversation.displayName) }}</span>
        </div>
        <div class="title-info">
          <h2>{{ conversation.displayName }}</h2>
          <span v-if="isGroup" class="subtitle">{{ participantNames }} &bull; tap to manage</span>
        </div>
      </div>
      <div style="width:80px"></div>
    </div>

    <!-- Group management panel -->
    <div v-if="showGroupPanel && isGroup" class="group-panel">
      <div class="group-panel-section">
        <strong>Members:</strong>
        <div class="member-list">
          <span v-for="p in conversation.participants" :key="p.identifier" class="member-badge">
            <img v-if="p.photoUrl" :src="resolveUrl(p.photoUrl)" class="member-avatar" />
            <span v-else class="member-initial">{{ initial(p.username) }}</span>
            {{ p.username }}
          </span>
        </div>
      </div>
      <div class="group-panel-actions">
        <button class="btn btn-sm btn-secondary" @click="showAddMember = true">+ Add Member</button>
        <button class="btn btn-sm btn-secondary" @click="openRenameGroup">&#9998; Rename</button>
        <button class="btn btn-sm btn-secondary" @click="$refs.groupPhotoInput.click()">&#128247; Change Photo</button>
        <button class="btn btn-sm btn-danger" @click="confirmLeaveGroup">&#10060; Leave Group</button>
        <input ref="groupPhotoInput" type="file" accept="image/*" style="display:none" @change="uploadGroupPhoto" />
      </div>
    </div>

    <!-- Messages area -->
    <div class="messages-area" ref="msgArea" @click="activeMsg = null">
      <div v-if="loading" class="center-info"><div class="spinner"></div></div>
      <div v-else-if="messages.length === 0" class="center-info">
        <p>No messages yet. Say hello!</p>
      </div>
      <div v-else class="messages-list">
        <template v-for="msg in messages" :key="msg.messageId">
          <!-- System message: group event announcements -->
          <div v-if="msg.messageType === 'system'" class="system-msg">
            {{ msg.textContent }}
          </div>

          <div
            v-else
            :class="['msg-row', isMine(msg) ? 'mine' : 'theirs']"
          >
          <!-- Sender label for group received messages -->
          <div v-if="!isMine(msg) && isGroup" class="sender-label">{{ msg.senderName }}</div>

          <div class="bubble-wrapper" @click.stop="toggleActions(msg.messageId)">
            <!-- Forwarded badge -->
            <div v-if="msg.forwardedFromId" class="fwd-badge">&#8618; Forwarded</div>

            <!-- Reply preview -->
            <div v-if="msg.replyPreview" class="reply-preview-inline">
              <span class="reply-sender">{{ msg.replyPreview.senderName }}</span>
              <span class="reply-text">
                {{ msg.replyPreview.hasPhoto ? '&#128247; Photo' : (msg.replyPreview.contentPreview || '') }}
              </span>
            </div>

            <!-- Message content -->
            <div v-if="msg.textContent" class="msg-text">{{ msg.textContent }}</div>
            <img v-if="msg.photoUrl" :src="resolveUrl(msg.photoUrl)" class="msg-photo" />

            <!-- Meta line -->
            <div class="msg-meta">
              <span class="msg-time">{{ fmtTime(msg.sentAt) }}</span>
              <span v-if="isMine(msg)" class="checkmarks" :title="msg.deliveryStatus">
                <span v-if="msg.deliveryStatus === 'read'" class="checks read">&#10003;&#10003;</span>
                <span v-else-if="msg.deliveryStatus === 'received'" class="checks received">&#10003;</span>
                <!-- no checkmark for 'sent' -->
              </span>
            </div>

            <!-- Reactions display -->
            <div v-if="msg.reactions && msg.reactions.length" class="reactions-row">
              <span
                v-for="r in msg.reactions" :key="r.reactionId"
                class="reaction-chip"
                :class="{ mine: r.userId === currentUserId }"
                :title="r.username"
                @click.stop="r.userId === currentUserId ? removeMyReaction(msg.messageId, r.reactionId) : null"
              >{{ r.emoji }} <small>{{ r.username }}</small></span>
            </div>
          </div>

          <!-- Action toolbar (appears on click) -->
          <div v-if="activeMsg === msg.messageId" class="msg-actions" @click.stop>
            <button class="act-btn" @click="startReply(msg)" title="Reply">&#8629;</button>
            <button class="act-btn" @click="startForward(msg)" title="Forward">&#8618;</button>
            <button class="act-btn" @click="openEmojiPicker(msg.messageId)" title="React">&#128512;</button>
            <button v-if="isMine(msg)" class="act-btn danger" @click="deleteMsg(msg.messageId)" title="Delete">&#128465;</button>
          </div>

          <!-- Emoji picker -->
          <div v-if="emojiPickerFor === msg.messageId" class="emoji-picker" @click.stop>
            <span v-for="e in emojis" :key="e" class="emoji-opt" @click="react(msg.messageId, e)">{{ e }}</span>
            <button class="emoji-close" @click="emojiPickerFor = null">&#x2715;</button>
          </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Reply banner -->
    <div v-if="replyingTo" class="reply-banner">
      <div class="reply-banner-content">
        <span class="reply-label">Replying to <strong>{{ replyingTo.senderName }}</strong></span>
        <span class="reply-snippet">{{ replyingTo.hasPhoto ? '&#128247; Photo' : replyingTo.textContent }}</span>
      </div>
      <button class="reply-close" @click="replyingTo = null">&#x2715;</button>
    </div>

    <!-- Selected file preview -->
    <div v-if="selectedFile" class="file-preview">
      <span>&#128247; {{ selectedFile.name }}</span>
      <button @click="selectedFile = null">&#x2715;</button>
    </div>

    <!-- Message input -->
    <div class="input-row">
      <input
        v-model="draft"
        class="msg-input"
        placeholder="Type a message..."
        @keydown.enter.exact.prevent="send"
      />
      <input ref="photoFile" type="file" accept="image/*" style="display:none" @change="onFileSelected" />
      <button class="btn btn-icon" @click="$refs.photoFile.click()" title="Attach photo">&#128247;</button>
      <button class="btn btn-primary" :disabled="!draft.trim() && !selectedFile" @click="send">Send</button>
    </div>

    <!-- Forward modal -->
    <div v-if="forwardingMsg" class="modal-overlay" @click.self="forwardingMsg = null">
      <div class="modal">
        <div class="modal-hdr">
          <h3>Forward to...</h3>
          <button @click="forwardingMsg = null">&#x2715;</button>
        </div>
        <div class="modal-body">
          <div v-if="conversations.length === 0" class="muted">No conversations yet</div>
          <div
            v-for="c in conversations"
            :key="c.conversationId"
            class="conv-pick"
            @click="doForward(c.conversationId)"
          >
            <div class="avatar avatar-xs" :style="avatarStyle(c.displayName)">
              <img v-if="c.displayPhotoUrl" :src="resolveUrl(c.displayPhotoUrl)" class="avatar-img" />
              <span v-else>{{ initial(c.displayName) }}</span>
            </div>
            <span>{{ c.displayName }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Add member modal -->
    <div v-if="showAddMember" class="modal-overlay" @click.self="showAddMember = false">
      <div class="modal">
        <div class="modal-hdr">
          <h3>Add Member</h3>
          <button @click="showAddMember = false">&#x2715;</button>
        </div>
        <div class="modal-body">
          <input v-model="memberSearch" class="input" placeholder="Search user..." @input="searchForMember" />
          <div class="search-results">
            <div
              v-for="u in memberResults"
              :key="u.identifier"
              class="conv-pick"
              @click="addMember(u.identifier)"
            >
              <div class="avatar avatar-xs" :style="avatarStyle(u.username)">
                <img v-if="u.photoUrl" :src="resolveUrl(u.photoUrl)" class="avatar-img" />
                <span v-else>{{ initial(u.username) }}</span>
              </div>
              <span>{{ u.username }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Rename group modal -->
    <div v-if="showRename" class="modal-overlay" @click.self="showRename = false">
      <div class="modal">
        <div class="modal-hdr">
          <h3>Rename Group</h3>
          <button @click="showRename = false">&#x2715;</button>
        </div>
        <div class="modal-body">
          <input v-model="newGroupName" class="input" placeholder="New group name" maxlength="100" />
        </div>
        <div class="modal-ftr">
          <button class="btn btn-secondary" @click="showRename = false">Cancel</button>
          <button class="btn btn-primary" @click="doRename">Rename</button>
        </div>
      </div>
    </div>

    <!-- Confirmation modal -->
    <div v-if="confirmDialog.show" class="modal-overlay" @click.self="resolveConfirm(false)">
      <div class="modal">
        <div class="modal-hdr">
          <h3>Please confirm</h3>
          <button @click="resolveConfirm(false)">&#x2715;</button>
        </div>
        <div class="modal-body">
          <p>{{ confirmDialog.message }}</p>
        </div>
        <div class="modal-ftr">
          <button class="btn btn-secondary" @click="resolveConfirm(false)">Cancel</button>
          <button class="btn btn-danger" @click="resolveConfirm(true)">Confirm</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import {
  getConversation, sendMessage as apiSend, deleteMessage as apiDelete,
  forwardMessage as apiForward, addReaction as apiReact, removeReaction as apiUnreact,
  getConversations, searchUsers, addGroupMember as apiAddMember, leaveGroup as apiLeave,
  updateGroupName as apiRename, uploadGroupPhoto as apiGroupPhoto
} from '../services/api'
import { getUserId, getUsername } from '../services/auth'
import { showToast } from '../services/toast'
import { API_URL } from '../services/config'

export default {
  name: 'ChatView',
  data() {
    return {
      conversation: null,
      messages: [],
      loading: true,
      draft: '',
      selectedFile: null,
      currentUserId: getUserId(),
      currentUsername: getUsername(),
      timer: null,

      activeMsg: null,
      emojiPickerFor: null,
      emojis: ['👍','❤️','😂','😮','😢','😡','🎉','👏','🔥','💯'],

      replyingTo: null,
      forwardingMsg: null,
      conversations: [],

      showGroupPanel: false,
      showAddMember: false,
      memberSearch: '',
      memberResults: [],
      showRename: false,
      newGroupName: '',

      confirmDialog: { show: false, message: '', onConfirm: null },
    }
  },
  computed: {
    isGroup() {
      return this.conversation?.conversationType === 'group'
    },
    participantNames() {
      if (!this.conversation?.participants) return ''
      return this.conversation.participants.map(p => p.username).join(', ')
    },
    headerAvatarStyle() {
      return this.avatarStyle(this.conversation?.displayName || '')
    }
  },
  mounted() {
    this.load()
    this.timer = setInterval(this.silentRefresh, 2500)
  },
  beforeUnmount() {
    clearInterval(this.timer)
  },
  methods: {
    async load() {
      this.loading = true
      try {
        const convId = this.$route.params.conversationId
        const data = await getConversation(convId)
        this.conversation = data
        this.messages = this.sorted(data.messages || [])
        this.$nextTick(this.scrollBottom)
      } catch (e) {
        console.error(e)
      } finally {
        this.loading = false
      }
    },
    async silentRefresh() {
      if (this.loading) return
      try {
        const convId = this.$route.params.conversationId
        const data = await getConversation(convId)
        this.conversation = data
        const sorted = this.sorted(data.messages || [])
        const prevLen = this.messages.length
        const prevLast = this.messages[this.messages.length - 1]?.messageId
        const newLast = sorted[sorted.length - 1]?.messageId

        this.messages = sorted

        if (sorted.length > prevLen || newLast !== prevLast) {
          this.$nextTick(() => {
            const el = this.$refs.msgArea
            if (el && el.scrollHeight - el.scrollTop - el.clientHeight < 120) {
              this.scrollBottom()
            }
          })
        }
      } catch (e) { /* silent */ }
    },
    sorted(msgs) {
      return [...msgs].sort((a, b) => new Date(a.sentAt) - new Date(b.sentAt))
    },
    async send() {
      if (!this.draft.trim() && !this.selectedFile) return
      const convId = this.$route.params.conversationId
      const replyId = this.replyingTo?.messageId || null
      try {
        const msg = await apiSend(convId, this.draft.trim(), this.selectedFile, replyId)
        this.messages.push(msg)
        this.draft = ''
        this.selectedFile = null
        this.replyingTo = null
        this.$nextTick(this.scrollBottom)
        this.silentRefresh()
      } catch (e) {
        showToast('Failed to send message: ' + (e.message || ''), 'error')
      }
    },
    onFileSelected(e) {
      const f = e.target.files[0]
      if (f) this.selectedFile = f
      e.target.value = ''
    },
    isMine(msg) {
      return msg.senderId === this.currentUserId
    },
    toggleActions(id) {
      this.emojiPickerFor = null
      this.activeMsg = this.activeMsg === id ? null : id
    },
    openEmojiPicker(id) {
      this.activeMsg = null
      this.emojiPickerFor = id
    },
    startReply(msg) {
      this.replyingTo = {
        messageId: msg.messageId,
        senderName: msg.senderName,
        textContent: msg.textContent || '',
        hasPhoto: !!msg.photoUrl
      }
      this.activeMsg = null
    },
    async startForward(msg) {
      this.forwardingMsg = msg
      this.activeMsg = null
      try {
        const data = await getConversations()
        this.conversations = (data.conversations || []).filter(c => c.conversationId !== this.$route.params.conversationId)
      } catch (e) { this.conversations = [] }
    },
    async doForward(targetId) {
      if (!this.forwardingMsg) return
      try {
        await apiForward(this.forwardingMsg.messageId, targetId)
        this.forwardingMsg = null
        showToast('Message forwarded!', 'success')
      } catch (e) {
        showToast('Failed to forward: ' + (e.message || ''), 'error')
      }
    },
    async react(msgId, emoji) {
      this.emojiPickerFor = null
      try {
        await apiReact(msgId, emoji)
        await this.silentRefresh()
      } catch (e) {
        showToast('Failed to react: ' + (e.message || ''), 'error')
      }
    },
    async removeMyReaction(msgId, reactionId) {
      try {
        await apiUnreact(msgId, reactionId)
        await this.silentRefresh()
      } catch (e) {
        showToast('Failed to remove reaction: ' + (e.message || ''), 'error')
      }
    },
    deleteMsg(id) {
      this.askConfirm('Delete this message?', async () => {
        try {
          await apiDelete(id)
          this.messages = this.messages.filter(m => m.messageId !== id)
          this.activeMsg = null
        } catch (e) {
          showToast('Failed to delete: ' + (e.message || ''), 'error')
        }
      })
    },
    openRenameGroup() {
      this.newGroupName = this.conversation?.displayName || ''
      this.showRename = true
    },
    async doRename() {
      if (!this.newGroupName.trim()) return
      const convId = this.$route.params.conversationId
      try {
        await apiRename(convId, this.newGroupName.trim())
        this.showRename = false
        await this.silentRefresh()
      } catch (e) {
        showToast('Failed to rename: ' + (e.message || ''), 'error')
      }
    },
    async uploadGroupPhoto(e) {
      const f = e.target.files[0]
      if (!f) return
      const convId = this.$route.params.conversationId
      try {
        await apiGroupPhoto(convId, f)
        await this.silentRefresh()
      } catch (e) {
        showToast('Failed to update photo: ' + (e.message || ''), 'error')
      }
      e.target.value = ''
    },
    async searchForMember() {
      try {
        const data = await searchUsers(this.memberSearch)
        const existing = new Set((this.conversation?.participants || []).map(p => p.identifier))
        this.memberResults = (data.users || []).filter(u => !existing.has(u.identifier))
      } catch (e) { this.memberResults = [] }
    },
    async addMember(userId) {
      const convId = this.$route.params.conversationId
      try {
        await apiAddMember(convId, userId)
        this.showAddMember = false
        this.memberSearch = ''
        this.memberResults = []
        await this.load()
        showToast('Member added!', 'success')
      } catch (e) {
        showToast('Failed to add member: ' + (e.message || ''), 'error')
      }
    },
    confirmLeaveGroup() {
      this.askConfirm('Leave this group?', async () => {
        const convId = this.$route.params.conversationId
        try {
          await apiLeave(convId)
          this.$router.push('/')
        } catch (e) {
          showToast('Failed to leave group: ' + (e.message || ''), 'error')
        }
      })
    },
    askConfirm(message, onConfirm) {
      this.confirmDialog = { show: true, message, onConfirm }
    },
    resolveConfirm(result) {
      const cb = this.confirmDialog.onConfirm
      this.confirmDialog = { show: false, message: '', onConfirm: null }
      if (result && cb) cb()
    },
    resolveUrl(url) {
      if (!url) return ''
      if (url.startsWith('http')) return url
      return API_URL + url
    },
    initial(name) {
      return name ? name.charAt(0).toUpperCase() : '?'
    },
    avatarStyle(name) {
      const colors = ['#6366f1','#8b5cf6','#ec4899','#ef4444','#f59e0b','#10b981','#06b6d4','#3b82f6']
      const i = name ? name.charCodeAt(0) % colors.length : 0
      return { background: colors[i] }
    },
    fmtTime(ts) {
      const d = new Date(ts)
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    },
    scrollBottom() {
      const el = this.$refs.msgArea
      if (el) el.scrollTop = el.scrollHeight
    },
    goBack() {
      this.$router.push('/')
    }
  }
}
</script>

<style scoped>
.chat-view {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f0f2f5;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}

/* Header */
.chat-header {
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
  padding: 12px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 1px 3px rgba(0,0,0,.08);
  z-index: 20;
}
.chat-title { display: flex; align-items: center; gap: 10px; }
.chat-title.clickable { cursor: pointer; }
.title-info h2 { font-size: 17px; font-weight: 600; margin: 0; }
.subtitle { font-size: 12px; color: #6b7280; }

/* Group panel */
.group-panel {
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
  padding: 12px 16px;
  font-size: 14px;
}
.group-panel-section { margin-bottom: 10px; }
.member-list { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 6px; }
.member-badge {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 10px; background: #f3f4f6; border-radius: 20px; font-size: 13px;
}
.member-avatar { width: 20px; height: 20px; border-radius: 50%; object-fit: cover; }
.member-initial {
  width: 20px; height: 20px; border-radius: 50%; background: #6366f1;
  color: #fff; font-size: 11px; display: inline-flex; align-items: center; justify-content: center;
}
.group-panel-actions { display: flex; flex-wrap: wrap; gap: 8px; }

/* Messages */
.messages-area {
  flex: 1; overflow-y: auto; padding: 16px;
  display: flex; flex-direction: column;
}
.center-info { display: flex; align-items: center; justify-content: center; flex: 1; color: #6b7280; font-size: 15px; }
.messages-list { display: flex; flex-direction: column; gap: 6px; }
.spinner {
  width: 36px; height: 36px; border: 4px solid #e5e7eb;
  border-top-color: #6366f1; border-radius: 50%; animation: spin .8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.msg-row { display: flex; flex-direction: column; max-width: 75%; position: relative; }
.msg-row.mine { align-self: flex-end; align-items: flex-end; }
.msg-row.theirs { align-self: flex-start; align-items: flex-start; }

.system-msg {
  align-self: center;
  text-align: center;
  color: #6b7280;
  font-size: 12.5px;
  background: rgba(0,0,0,.04);
  padding: 4px 12px;
  border-radius: 12px;
  margin: 4px 0;
}

.sender-label { font-size: 12px; color: #6366f1; font-weight: 600; margin-bottom: 2px; padding-left: 4px; }

.bubble-wrapper {
  background: #fff; border-radius: 16px; padding: 10px 14px;
  box-shadow: 0 1px 2px rgba(0,0,0,.1); cursor: pointer;
  transition: box-shadow .15s; max-width: 100%;
}
.bubble-wrapper:hover { box-shadow: 0 2px 8px rgba(0,0,0,.15); }
.mine .bubble-wrapper { background: linear-gradient(135deg, #6366f1, #818cf8); color: #fff; }

.fwd-badge {
  font-size: 11px; font-style: italic; opacity: .7; margin-bottom: 4px;
  display: flex; align-items: center; gap: 4px;
}

.reply-preview-inline {
  background: rgba(0,0,0,.07); border-left: 3px solid #6366f1;
  border-radius: 6px; padding: 4px 8px; margin-bottom: 6px; font-size: 13px;
}
.mine .reply-preview-inline { background: rgba(255,255,255,.2); border-left-color: rgba(255,255,255,.8); }
.reply-sender { font-weight: 600; display: block; font-size: 12px; }
.reply-text { color: inherit; opacity: .85; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block; }

.msg-text { word-break: break-word; line-height: 1.5; font-size: 15px; }
.msg-photo { max-width: 260px; max-height: 320px; border-radius: 10px; margin-top: 6px; object-fit: cover; display: block; }

.msg-meta { display: flex; justify-content: flex-end; align-items: center; gap: 6px; margin-top: 4px; font-size: 11px; opacity: .7; }
.mine .msg-meta { color: rgba(255,255,255,.9); }
.checkmarks { font-size: 13px; font-weight: 700; }
.checks.read { color: #34d399; }
.checks.received { color: inherit; }
.mine .checks.read { color: #a7f3d0; }

.reactions-row { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 6px; }
.reaction-chip {
  display: inline-flex; align-items: center; gap: 3px;
  background: rgba(0,0,0,.06); border-radius: 12px;
  padding: 2px 8px; font-size: 14px; cursor: default;
}
.reaction-chip.mine { background: rgba(99,102,241,.15); cursor: pointer; }
.reaction-chip small { font-size: 11px; color: #6b7280; }

/* Action bar */
.msg-actions {
  display: flex; gap: 6px; margin-top: 4px;
  background: #fff; border-radius: 20px; padding: 4px 8px;
  box-shadow: 0 2px 8px rgba(0,0,0,.15);
}
.mine .msg-actions { flex-direction: row-reverse; }
.act-btn {
  background: none; border: none; font-size: 18px; cursor: pointer; padding: 4px 6px;
  border-radius: 8px; transition: background .15s;
}
.act-btn:hover { background: #f3f4f6; }
.act-btn.danger:hover { background: #fee2e2; color: #ef4444; }

/* Emoji picker */
.emoji-picker {
  display: flex; flex-wrap: wrap; gap: 4px; padding: 8px 10px;
  background: #fff; border-radius: 16px; box-shadow: 0 4px 16px rgba(0,0,0,.15);
  margin-top: 4px; max-width: 280px;
}
.emoji-opt { font-size: 22px; cursor: pointer; padding: 4px; border-radius: 8px; transition: transform .1s; }
.emoji-opt:hover { transform: scale(1.3); background: #f3f4f6; }
.emoji-close { background: none; border: none; color: #6b7280; cursor: pointer; font-size: 16px; padding: 4px 6px; }

/* Reply banner */
.reply-banner {
  background: #eef2ff; border-top: 1px solid #c7d2fe;
  padding: 8px 16px; display: flex; align-items: center; justify-content: space-between;
}
.reply-banner-content { display: flex; flex-direction: column; }
.reply-label { font-size: 12px; color: #6366f1; font-weight: 600; }
.reply-snippet { font-size: 13px; color: #374151; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 300px; }
.reply-close { background: none; border: none; color: #6b7280; font-size: 18px; cursor: pointer; }

/* File preview */
.file-preview {
  background: #f3f4f6; border-top: 1px solid #e5e7eb;
  padding: 6px 16px; display: flex; align-items: center; justify-content: space-between;
  font-size: 13px; color: #374151;
}
.file-preview button { background: none; border: none; cursor: pointer; color: #6b7280; font-size: 16px; }

/* Input row */
.input-row {
  background: #fff; border-top: 1px solid #e5e7eb;
  padding: 12px 16px; display: flex; gap: 8px; align-items: center;
}
.msg-input {
  flex: 1; border: 1px solid #d1d5db; border-radius: 24px;
  padding: 10px 16px; font-size: 15px; outline: none; background: #f9fafb;
  transition: border-color .15s;
}
.msg-input:focus { border-color: #6366f1; background: #fff; }

/* Buttons */
.btn { padding: 8px 16px; border-radius: 8px; border: none; cursor: pointer; font-size: 14px; font-weight: 500; transition: all .15s; }
.btn-primary { background: #6366f1; color: #fff; }
.btn-primary:hover { background: #4f46e5; }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-secondary { background: #f3f4f6; color: #374151; border: 1px solid #e5e7eb; }
.btn-secondary:hover { background: #e5e7eb; }
.btn-danger { background: #fee2e2; color: #ef4444; border: 1px solid #fca5a5; }
.btn-danger:hover { background: #fca5a5; }
.btn-icon { background: none; border: none; cursor: pointer; font-size: 20px; color: #6b7280; padding: 6px; border-radius: 8px; }
.btn-icon:hover { background: #f3f4f6; }
.btn-sm { padding: 5px 10px; font-size: 12px; border-radius: 6px; }

/* Avatar */
.avatar {
  width: 40px; height: 40px; border-radius: 50%; display: flex; align-items: center;
  justify-content: center; font-weight: 600; color: #fff; font-size: 16px; overflow: hidden; flex-shrink: 0;
}
.avatar-sm { width: 36px; height: 36px; font-size: 14px; }
.avatar-xs { width: 28px; height: 28px; font-size: 12px; }
.avatar-img { width: 100%; height: 100%; object-fit: cover; }

/* Modal */
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,.4);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal {
  background: #fff; border-radius: 16px; width: 360px; max-width: 95vw;
  box-shadow: 0 20px 60px rgba(0,0,0,.3); overflow: hidden; display: flex; flex-direction: column;
}
.modal-hdr {
  padding: 16px 20px; display: flex; justify-content: space-between; align-items: center;
  border-bottom: 1px solid #e5e7eb;
}
.modal-hdr h3 { font-size: 18px; font-weight: 600; margin: 0; }
.modal-hdr button { background: none; border: none; font-size: 20px; cursor: pointer; color: #6b7280; }
.modal-body { padding: 16px 20px; max-height: 400px; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; }
.modal-ftr { padding: 12px 20px; border-top: 1px solid #e5e7eb; display: flex; justify-content: flex-end; gap: 8px; }

.input { border: 1px solid #d1d5db; border-radius: 8px; padding: 8px 12px; font-size: 14px; width: 100%; outline: none; }
.input:focus { border-color: #6366f1; }

.conv-pick {
  display: flex; align-items: center; gap: 10px; padding: 10px; border-radius: 10px;
  cursor: pointer; transition: background .15s;
}
.conv-pick:hover { background: #f3f4f6; }

.search-results { display: flex; flex-direction: column; gap: 4px; margin-top: 8px; }
.muted { color: #9ca3af; font-size: 14px; text-align: center; padding: 16px 0; }
</style>
