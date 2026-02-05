<template>
  <div class="home-view">
    <div class="app-header">
      <div class="header-content">
        <h1>WASAText</h1>
        <div class="header-actions">
          <button @click="showNewChatModal = true" class="btn btn-primary">New Chat</button>
          <button @click="showNewGroupModal = true" class="btn btn-secondary">New Group</button>
          <button @click="goToProfile" class="btn btn-secondary">Profile</button>
          <button @click="handleLogout" class="btn btn-secondary">Logout</button>
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
            <div class="avatar">{{ getInitial(conv.displayName) }}</div>
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
              <div class="avatar avatar-sm">{{ getInitial(user.username) }}</div>
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
              <div class="avatar avatar-sm">{{ getInitial(user.username) }}</div>
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
  background-color: var(--secondary-color);
}

.app-header {
  background: var(--bg-white);
  border-bottom: 1px solid var(--border-color);
  padding: 16px 24px;
  box-shadow: var(--shadow);
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-content h1 {
  font-size: 24px;
  color: var(--primary-color);
}

.header-actions {
  display: flex;
  gap: 10px;
}

.conversations-container {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

.conversations-list {
  max-width: 800px;
  margin: 0 auto;
}

.conversation-item {
  background: var(--bg-white);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 12px;
  display: flex;
  gap: 12px;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: var(--shadow);
}

.conversation-item:hover {
  background: var(--bg-hover);
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg);
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
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.conversation-time {
  font-size: 12px;
  color: var(--text-secondary);
}

.conversation-preview {
  font-size: 14px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--text-secondary);
}

.empty-state-subtitle {
  margin-top: 8px;
  font-size: 14px;
}

.search-results {
  margin-top: 12px;
  max-height: 300px;
  overflow-y: auto;
}

.search-result-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}

.search-result-item:hover {
  background: var(--bg-hover);
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
  gap: 6px;
  padding: 6px 12px;
  background: var(--primary-color);
  color: white;
  border-radius: 16px;
  font-size: 14px;
}

.selected-member-tag button {
  background: none;
  border: none;
  color: white;
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
}
</style>
