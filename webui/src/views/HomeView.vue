<template>
  <div class="home-view">
    <div class="app-header">
      <h1 class="logo">WASAText</h1>
      <div class="header-actions">
        <button @click="showNewChat = true" class="btn btn-primary">&#128172; New Chat</button>
        <button @click="showNewGroup = true" class="btn btn-secondary">&#128101; New Group</button>
        <button @click="$router.push('/profile')" class="btn btn-secondary">&#9881; Profile</button>
        <button @click="handleLogout" class="btn btn-secondary">&#128682; Logout</button>
      </div>
    </div>

    <div class="conv-list-area">
      <div v-if="loading" class="center-msg"><div class="spinner"></div></div>
      <div v-else-if="conversations.length === 0" class="center-msg empty">
        <div class="empty-icon">&#128172;</div>
        <p>No conversations yet</p>
        <p class="sub">Start a new chat or create a group!</p>
      </div>
      <div v-else class="conv-list">
        <div
          v-for="conv in conversations"
          :key="conv.conversationId"
          class="conv-item"
          @click="open(conv.conversationId)"
        >
          <div class="avatar" :style="avatarStyle(conv.displayName)">
            <img v-if="conv.displayPhotoUrl" :src="resolveUrl(conv.displayPhotoUrl)" class="avatar-img" />
            <span v-else>{{ initial(conv.displayName) }}</span>
          </div>
          <div class="conv-info">
            <div class="conv-top">
              <span class="conv-name">{{ conv.displayName }}</span>
              <span class="conv-time">{{ fmtTime(conv.lastMessageTimestamp) }}</span>
            </div>
            <div class="conv-preview">
              <span v-if="conv.lastMessageIsPhoto">&#128247; Photo</span>
              <span v-else>{{ conv.lastMessageSnippet || 'No messages yet' }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- New Chat Modal -->
    <div v-if="showNewChat" class="modal-overlay" @click.self="showNewChat = false">
      <div class="modal">
        <div class="modal-hdr">
          <h3>New Conversation</h3>
          <button @click="showNewChat = false">&#x2715;</button>
        </div>
        <div class="modal-body">
          <input
            v-model="chatSearch"
            class="input"
            placeholder="Search users..."
            @input="searchUsers"
            @focus="searchUsers"
          />
          <div class="result-list">
            <div
              v-for="u in chatResults"
              :key="u.identifier"
              class="result-item"
              @click="startChat(u.identifier)"
            >
              <div class="avatar avatar-sm" :style="avatarStyle(u.username)">
                <img v-if="u.photoUrl" :src="resolveUrl(u.photoUrl)" class="avatar-img" />
                <span v-else>{{ initial(u.username) }}</span>
              </div>
              <span>{{ u.username }}</span>
            </div>
            <div v-if="chatResults.length === 0 && chatSearch" class="no-results">No users found</div>
          </div>
        </div>
      </div>
    </div>

    <!-- New Group Modal -->
    <div v-if="showNewGroup" class="modal-overlay" @click.self="closeGroupModal">
      <div class="modal">
        <div class="modal-hdr">
          <h3>Create Group</h3>
          <button @click="closeGroupModal">&#x2715;</button>
        </div>
        <div class="modal-body">
          <input v-model="groupName" class="input" placeholder="Group name" maxlength="100" />
          <input
            v-model="groupMemberSearch"
            class="input"
            placeholder="Search members to add..."
            @input="searchGroupMembers"
            @focus="searchGroupMembers"
            style="margin-top:10px"
          />
          <div class="result-list">
            <div
              v-for="u in groupMemberResults"
              :key="u.identifier"
              class="result-item"
              :class="{ selected: isSelected(u.identifier) }"
              @click="toggleMember(u)"
            >
              <div class="avatar avatar-sm" :style="avatarStyle(u.username)">
                <img v-if="u.photoUrl" :src="resolveUrl(u.photoUrl)" class="avatar-img" />
                <span v-else>{{ initial(u.username) }}</span>
              </div>
              <span>{{ u.username }}</span>
              <span v-if="isSelected(u.identifier)" class="check-mark">&#10003;</span>
            </div>
          </div>
          <div v-if="selectedMembers.length" class="selected-tags">
            <span v-for="m in selectedMembers" :key="m.identifier" class="tag">
              {{ m.username }}
              <button @click="removeMember(m.identifier)">&#x2715;</button>
            </span>
          </div>
        </div>
        <div class="modal-ftr">
          <button @click="closeGroupModal" class="btn btn-secondary">Cancel</button>
          <button @click="createGroup" class="btn btn-primary" :disabled="!groupName.trim()">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { getConversations, searchUsers as apiSearch, createConversation, createGroup as apiCreateGroup } from '../services/api'
import { logout } from '../services/auth'
import { API_URL } from '../services/config'

export default {
  name: 'HomeView',
  data() {
    return {
      conversations: [],
      loading: true,
      timer: null,

      showNewChat: false,
      chatSearch: '',
      chatResults: [],

      showNewGroup: false,
      groupName: '',
      groupMemberSearch: '',
      groupMemberResults: [],
      selectedMembers: [],
    }
  },
  mounted() {
    this.load()
    this.timer = setInterval(this.silentRefresh, 3000)
  },
  beforeUnmount() {
    clearInterval(this.timer)
  },
  methods: {
    async load() {
      this.loading = true
      try {
        const data = await getConversations()
        this.conversations = data.conversations || []
      } catch (e) {
        console.error('Failed to load conversations:', e)
      } finally {
        this.loading = false
      }
    },
    async silentRefresh() {
      try {
        const data = await getConversations()
        this.conversations = data.conversations || []
      } catch (e) { /* silent */ }
    },
    open(id) {
      this.$router.push(`/chat/${id}`)
    },
    handleLogout() {
      logout()
    },
    async searchUsers() {
      try {
        const data = await apiSearch(this.chatSearch)
        this.chatResults = data.users || []
      } catch (e) { this.chatResults = [] }
    },
    async startChat(userId) {
      try {
        const conv = await createConversation(userId)
        this.showNewChat = false
        this.chatSearch = ''
        this.chatResults = []
        this.$router.push(`/chat/${conv.conversationId}`)
      } catch (e) {
        alert('Failed to start conversation: ' + (e.message || ''))
      }
    },
    async searchGroupMembers() {
      try {
        const data = await apiSearch(this.groupMemberSearch)
        this.groupMemberResults = data.users || []
      } catch (e) { this.groupMemberResults = [] }
    },
    isSelected(id) {
      return this.selectedMembers.some(m => m.identifier === id)
    },
    toggleMember(user) {
      const idx = this.selectedMembers.findIndex(m => m.identifier === user.identifier)
      if (idx === -1) this.selectedMembers.push(user)
      else this.selectedMembers.splice(idx, 1)
    },
    removeMember(id) {
      this.selectedMembers = this.selectedMembers.filter(m => m.identifier !== id)
    },
    async createGroup() {
      if (!this.groupName.trim()) { alert('Enter a group name'); return }
      try {
        const ids = this.selectedMembers.map(m => m.identifier)
        const g = await apiCreateGroup(this.groupName.trim(), ids)
        this.closeGroupModal()
        this.$router.push(`/chat/${g.groupId}`)
      } catch (e) {
        alert('Failed to create group: ' + (e.message || ''))
      }
    },
    closeGroupModal() {
      this.showNewGroup = false
      this.groupName = ''
      this.groupMemberSearch = ''
      this.groupMemberResults = []
      this.selectedMembers = []
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
      const now = new Date()
      if (now - d < 86400000) {
        return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      }
      return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
    }
  }
}
</script>

<style scoped>
.home-view {
  height: 100vh; display: flex; flex-direction: column; background: #f0f2f5;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}
.app-header {
  background: #fff; border-bottom: 1px solid #e5e7eb;
  padding: 14px 24px; display: flex; justify-content: space-between;
  align-items: center; box-shadow: 0 1px 3px rgba(0,0,0,.08); position: sticky; top: 0; z-index: 10;
}
.logo { font-size: 26px; font-weight: 700; color: #6366f1; margin: 0; }
.header-actions { display: flex; gap: 10px; flex-wrap: wrap; }
.conv-list-area { flex: 1; overflow-y: auto; padding: 16px; }
.center-msg { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; gap: 8px; }
.empty-icon { font-size: 48px; }
.empty p { font-size: 18px; color: #374151; margin: 0; }
.empty .sub { font-size: 14px; color: #9ca3af; }
.spinner {
  width: 36px; height: 36px; border: 4px solid #e5e7eb;
  border-top-color: #6366f1; border-radius: 50%; animation: spin .8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.conv-list { display: flex; flex-direction: column; gap: 8px; max-width: 800px; margin: 0 auto; }
.conv-item {
  background: #fff; border-radius: 12px; padding: 14px 16px;
  display: flex; gap: 14px; cursor: pointer;
  box-shadow: 0 1px 3px rgba(0,0,0,.06); transition: all .2s;
  border: 1px solid transparent;
}
.conv-item:hover { box-shadow: 0 4px 12px rgba(0,0,0,.1); border-color: #e0e7ff; transform: translateY(-1px); }
.conv-info { flex: 1; min-width: 0; }
.conv-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 3px; }
.conv-name { font-size: 16px; font-weight: 600; color: #111827; }
.conv-time { font-size: 12px; color: #9ca3af; white-space: nowrap; }
.conv-preview { font-size: 14px; color: #6b7280; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* Modal */
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,.4);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal {
  background: #fff; border-radius: 16px; width: 380px; max-width: 95vw;
  box-shadow: 0 20px 60px rgba(0,0,0,.25); overflow: hidden; display: flex; flex-direction: column;
}
.modal-hdr {
  padding: 16px 20px; display: flex; justify-content: space-between; align-items: center;
  border-bottom: 1px solid #e5e7eb;
}
.modal-hdr h3 { font-size: 18px; font-weight: 600; margin: 0; }
.modal-hdr button { background: none; border: none; font-size: 20px; cursor: pointer; color: #6b7280; }
.modal-body { padding: 16px 20px; max-height: 420px; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; }
.modal-ftr { padding: 12px 20px; border-top: 1px solid #e5e7eb; display: flex; justify-content: flex-end; gap: 8px; }

.input { border: 1px solid #d1d5db; border-radius: 8px; padding: 9px 12px; font-size: 14px; outline: none; width: 100%; box-sizing: border-box; }
.input:focus { border-color: #6366f1; }

.result-list { display: flex; flex-direction: column; gap: 4px; margin-top: 8px; }
.result-item {
  display: flex; align-items: center; gap: 10px; padding: 10px 12px;
  border-radius: 10px; cursor: pointer; transition: background .15s;
}
.result-item:hover { background: #f3f4f6; }
.result-item.selected { background: #eef2ff; }
.check-mark { margin-left: auto; color: #6366f1; font-weight: 700; }
.no-results { color: #9ca3af; font-size: 14px; text-align: center; padding: 12px; }

.selected-tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; padding-top: 10px; border-top: 1px solid #e5e7eb; }
.tag {
  display: inline-flex; align-items: center; gap: 6px; padding: 5px 12px;
  background: #6366f1; color: #fff; border-radius: 20px; font-size: 13px;
}
.tag button { background: rgba(255,255,255,.25); border: none; color: #fff; cursor: pointer; font-size: 13px; border-radius: 50%; width: 18px; height: 18px; display: flex; align-items: center; justify-content: center; }

/* Avatar */
.avatar {
  width: 42px; height: 42px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-weight: 700; color: #fff; font-size: 18px; overflow: hidden; flex-shrink: 0;
}
.avatar-sm { width: 32px; height: 32px; font-size: 13px; }
.avatar-img { width: 100%; height: 100%; object-fit: cover; }

/* Buttons */
.btn { padding: 8px 16px; border-radius: 8px; border: none; cursor: pointer; font-size: 14px; font-weight: 500; transition: all .15s; }
.btn-primary { background: #6366f1; color: #fff; }
.btn-primary:hover { background: #4f46e5; }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-secondary { background: #f3f4f6; color: #374151; border: 1px solid #e5e7eb; }
.btn-secondary:hover { background: #e5e7eb; }
</style>
