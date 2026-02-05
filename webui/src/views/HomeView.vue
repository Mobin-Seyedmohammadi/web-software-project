<template>
  <div class="home-view">
    <div class="app-header">
      <div class="header-content">
        <h1>WASAText</h1>
        <div class="header-actions">
          <button @click="showNewChatModal = true" class="btn btn-primary">💬 New Chat</button>
          <button @click="showNewGroupModal = true" class="btn btn-secondary">👥 New Group</button>
          <button @click="goToProfile" class="btn btn-secondary">⚙️ Profile</button>
          <button @click="handleLogout" class="btn btn-secondary">🚪 Logout</button>
        </div>
      </div>
    </div>

    <div class="conversations-container">
      <div v-if="loading" class="spinner"></div>

      <div v-else-if="conversations.length === 0" class="empty-state">
        <p>No conversations yet</p>
        <p class="empty-state-subtitle">Start a new conversation or create a group!</p>
      </div>

      <div v-else class="conversations-list">
        <div
          v-for="conv in conversations"
          :key="conv.conversationId"
          class="conversation-item"
          @click="openConversation(conv.conversationId)"
        >
          <div class="conversation-avatar">
            <div class="avatar" :style="{ background: getAvatarColor(conv.displayName) }">{{ getInitial(conv.displayName) }}</div>
          </div>
          <div class="conversation-content">
            <div class="conversation-header">
              <h3 class="conversation-name">{{ conv.displayName }}</h3>
              <span class="conversation-time">{{ formatTime(conv.lastMessageTimestamp) }}</span>
            </div>
            <div class="conversation-preview">
              <span v-if="conv.lastMessageIsPhoto">📷 Photo</span>
              <span v-else>{{ conv.lastMessageSnippet || 'No messages yet' }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- New Chat Modal -->
    <div v-if="showNewChatModal" class="modal-overlay" @click="showNewChatModal = false">
      <div class="modal" @click.stop>
        <div class="modal-header">
          <h2 class="modal-title">New Conversation</h2>
          <button class="modal-close" @click="showNewChatModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <input
            v-model="userSearchQuery"
            type="text"
            class="input"
            placeholder="Search users..."
            @input="searchUsers"
          />
          <div v-if="searchResults.length > 0" class="search-results">
            <div
              v-for="user in searchResults"
              :key="user.identifier"
              class="search-result-item"
              @click="startConversation(user.identifier)"
            >
              <div class="avatar avatar-sm" :style="{ background: getAvatarColor(user.username) }">{{ getInitial(user.username) }}</div>
              <span>{{ user.username }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- New Group Modal -->
    <div v-if="showNewGroupModal" class="modal-overlay" @click="showNewGroupModal = false">
      <div class="modal" @click.stop>
        <div class="modal-header">
          <h2 class="modal-title">Create Group</h2>
          <button class="modal-close" @click="showNewGroupModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Group Name</label>
            <input
              v-model="newGroupName"
              type="text"
              class="input"
              placeholder="Enter group name"
            />
          </div>
          <div class="form-group">
            <label class="form-label">Add Members</label>
            <input
              v-model="memberSearchQuery"
              type="text"
              class="input"
              placeholder="Search users..."
              @input="searchMembersForGroup"
            />
          </div>
          <div v-if="groupSearchResults.length > 0" class="search-results">
            <div
              v-for="user in groupSearchResults"
              :key="user.identifier"
              class="search-result-item"
              @click="toggleMember(user)"
            >
              <div class="avatar avatar-sm" :style="{ background: getAvatarColor(user.username) }">{{ getInitial(user.username) }}</div>
              <span>{{ user.username }}</span>
              <span v-if="isSelectedMember(user.identifier)">✓</span>
            </div>
          </div>
          <div v-if="selectedMembers.length > 0" class="selected-members">
            <h4>Selected Members:</h4>
            <div class="selected-member-list">
              <span
                v-for="member in selectedMembers"
                :key="member.identifier"
                class="selected-member-tag"
              >
                {{ member.username }}
                <button @click="removeMember(member.identifier)">&times;</button>
              </span>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button @click="showNewGroupModal = false" class="btn btn-secondary">Cancel</button>
          <button @click="createGroup" class="btn btn-primary">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { getConversations, searchUsers as apiSearchUsers, createConversation, createGroup } from '../services/api'
import { logout } from '../services/auth'

export default {
  name: 'HomeView',
  data() {
    return {
      conversations: [],
      loading: true,
      showNewChatModal: false,
      showNewGroupModal: false,
      userSearchQuery: '',
      memberSearchQuery: '',
      searchResults: [],
      groupSearchResults: [],
      newGroupName: '',
      selectedMembers: []
    }
  },
  mounted() {
    this.loadConversations()
  },
  methods: {
    async loadConversations() {
      try {
        this.loading = true
        const data = await getConversations()
        this.conversations = data.conversations || []
      } catch (error) {
        console.error('Failed to load conversations:', error)
      } finally {
        this.loading = false
      }
    },
    openConversation(conversationId) {
      this.$router.push(`/chat/${conversationId}`)
    },
    goToProfile() {
      this.$router.push('/profile')
    },
    handleLogout() {
      logout()
    },
    getInitial(name) {
      return name ? name.charAt(0).toUpperCase() : '?'
    },
    getAvatarColor(name) {
      const colors = [
        'linear-gradient(135deg, #6366f1 0%, #818cf8 100%)',
        'linear-gradient(135deg, #8b5cf6 0%, #a78bfa 100%)',
        'linear-gradient(135deg, #ec4899 0%, #f472b6 100%)',
        'linear-gradient(135deg, #ef4444 0%, #f87171 100%)',
        'linear-gradient(135deg, #f59e0b 0%, #fbbf24 100%)',
        'linear-gradient(135deg, #10b981 0%, #34d399 100%)',
        'linear-gradient(135deg, #06b6d4 0%, #22d3ee 100%)',
        'linear-gradient(135deg, #3b82f6 0%, #60a5fa 100%)'
      ]
      if (!name) return colors[0]
      const index = name.charCodeAt(0) % colors.length
      return colors[index]
    },
    formatTime(timestamp) {
      const date = new Date(timestamp)
      const now = new Date()
      const diff = now - date

      if (diff < 86400000) {
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      } else {
        return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
      }
    },
    async searchUsers() {
      if (this.userSearchQuery.length < 1) {
        this.searchResults = []
        return
      }

      try {
        const data = await apiSearchUsers(this.userSearchQuery)
        this.searchResults = data.users || []
      } catch (error) {
        console.error('Failed to search users:', error)
      }
    },
    async startConversation(userId) {
      try {
        const conversation = await createConversation(userId)
        this.showNewChatModal = false
        this.$router.push(`/chat/${conversation.conversationId}`)
      } catch (error) {
        console.error('Failed to create conversation:', error)
      }
    },
    async searchMembersForGroup() {
      if (this.memberSearchQuery.length < 1) {
        this.groupSearchResults = []
        return
      }

      try {
        const data = await apiSearchUsers(this.memberSearchQuery)
        this.groupSearchResults = data.users || []
      } catch (error) {
        console.error('Failed to search users:', error)
      }
    },
    toggleMember(user) {
      const index = this.selectedMembers.findIndex(m => m.identifier === user.identifier)
      if (index === -1) {
        this.selectedMembers.push(user)
      } else {
        this.selectedMembers.splice(index, 1)
      }
    },
    isSelectedMember(userId) {
      return this.selectedMembers.some(m => m.identifier === userId)
    },
    removeMember(userId) {
      const index = this.selectedMembers.findIndex(m => m.identifier === userId)
      if (index !== -1) {
        this.selectedMembers.splice(index, 1)
      }
    },
    async createGroup() {
      if (!this.newGroupName) {
        alert('Please enter a group name')
        return
      }

      try {
        const memberIds = this.selectedMembers.map(m => m.identifier)
        const group = await createGroup(this.newGroupName, memberIds)
        this.showNewGroupModal = false
        this.newGroupName = ''
        this.selectedMembers = []
        this.$router.push(`/chat/${group.groupId}`)
      } catch (error) {
        console.error('Failed to create group:', error)
        alert('Failed to create group')
      }
    }
  }
}
</script>

<style scoped>
.home-view {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
}

.app-header {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border-color);
  padding: 20px 32px;
  box-shadow: var(--shadow-md);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}

.header-content h1 {
  font-size: 28px;
  font-weight: 700;
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-light) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: -0.5px;
}

.header-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.conversations-container {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.conversations-list {
  max-width: 900px;
  margin: 0 auto;
}

.conversation-item {
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(10px);
  border-radius: var(--radius-lg);
  padding: 20px;
  margin-bottom: 16px;
  display: flex;
  gap: 16px;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: var(--shadow);
  border: 1px solid rgba(255, 255, 255, 0.8);
}

.conversation-item:hover {
  background: rgba(255, 255, 255, 1);
  transform: translateY(-4px) scale(1.01);
  box-shadow: var(--shadow-xl);
  border-color: var(--primary-light);
}

.conversation-content {
  flex: 1;
  min-width: 0;
}

.conversation-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.conversation-name {
  font-size: 17px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 2px;
}

.conversation-time {
  font-size: 13px;
  color: var(--text-muted);
  font-weight: 500;
  white-space: nowrap;
}

.conversation-preview {
  font-size: 14px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.4;
}

.empty-state {
  text-align: center;
  padding: 80px 20px;
  color: var(--text-secondary);
}

.empty-state p {
  font-size: 18px;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.empty-state-subtitle {
  margin-top: 8px;
  font-size: 15px;
  color: var(--text-muted);
}

.search-results {
  margin-top: 16px;
  max-height: 300px;
  overflow-y: auto;
  border-radius: var(--radius);
  background: var(--bg-hover);
  padding: 8px;
}

.search-result-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px;
  border-radius: var(--radius);
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  background: var(--bg-white);
  margin-bottom: 6px;
  box-shadow: var(--shadow-sm);
}

.search-result-item:hover {
  background: var(--primary-color);
  color: white;
  transform: translateX(4px);
  box-shadow: var(--shadow-md);
}

.search-result-item:last-child {
  margin-bottom: 0;
}

.selected-members {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border-color);
}

.selected-members h4 {
  font-size: 14px;
  margin-bottom: 8px;
}

.selected-member-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.selected-member-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-light) 100%);
  color: white;
  border-radius: 20px;
  font-size: 14px;
  font-weight: 500;
  box-shadow: var(--shadow-sm);
  transition: all 0.2s;
}

.selected-member-tag:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.selected-member-tag button {
  background: rgba(255, 255, 255, 0.2);
  border: none;
  color: white;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.selected-member-tag button:hover {
  background: rgba(255, 255, 255, 0.3);
}
</style>
