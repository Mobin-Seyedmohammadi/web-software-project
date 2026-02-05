<template>
  <div class="chat-view">
    <div class="chat-header">
      <button @click="goBack" class="btn btn-secondary">← Back</button>
      <div v-if="conversation" class="chat-title">
        <div class="avatar avatar-sm" :style="{ background: getAvatarColor(conversation.displayName) }">{{ getInitial(conversation.displayName) }}</div>
        <h2>{{ conversation.displayName }}</h2>
      </div>
      <div></div>
    </div>

    <div class="messages-container" ref="messagesContainer">
      <div v-if="loading" class="spinner"></div>
      <div v-else-if="messages.length === 0" class="empty-state">
        <p>No messages yet</p>
        <p class="empty-state-subtitle">Start the conversation!</p>
      </div>
      <div v-else class="messages-list">
        <div
          v-for="msg in messages"
          :key="msg.messageId"
          :class="['message', { 'message-sent': isSentByMe(msg), 'message-received': !isSentByMe(msg) }]"
        >
          <div v-if="!isSentByMe(msg)" class="message-sender">{{ msg.senderName }}</div>
          <div class="message-bubble">
            <div v-if="msg.textContent" class="message-text">{{ msg.textContent }}</div>
            <img v-if="msg.photoUrl" :src="getPhotoUrl(msg.photoUrl)" class="message-photo" />
            <div class="message-meta">
              <span class="message-time">{{ formatTime(msg.sentAt) }}</span>
              <span v-if="isSentByMe(msg)" class="message-status">{{ getStatusIcon(msg.deliveryStatus) }}</span>
            </div>
          </div>
          <div v-if="msg.reactions && msg.reactions.length > 0" class="message-reactions">
            <span v-for="reaction in msg.reactions" :key="reaction.reactionId" class="reaction">
              {{ reaction.emoji }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <div class="message-input-container">
      <input
        v-model="messageText"
        type="text"
        class="input message-input"
        placeholder="Type a message..."
        @keypress.enter="sendMessage"
      />
      <input
        ref="fileInput"
        type="file"
        accept="image/*"
        style="display: none"
        @change="handleFileSelect"
      />
      <button @click="$refs.fileInput.click()" class="btn btn-secondary" title="Attach photo">📷</button>
      <button @click="sendMessage" class="btn btn-primary" :disabled="!messageText.trim() && !selectedFile">
        <span v-if="messageText.trim() || selectedFile">Send</span>
        <span v-else>✉️</span>
      </button>
    </div>
  </div>
</template>

<script>
import { getConversation, sendMessage as apiSendMessage } from '../services/api'
import { getUsername } from '../services/auth'

export default {
  name: 'ChatView',
  data() {
    return {
      conversation: null,
      messages: [],
      loading: true,
      messageText: '',
      selectedFile: null,
      currentUsername: getUsername(),
      refreshInterval: null
    }
  },
  mounted() {
    this.loadConversation()
    // Set up auto-refresh every 2 seconds
    this.refreshInterval = setInterval(() => {
      this.checkForNewMessages()
    }, 2000)
  },
  beforeUnmount() {
    // Clean up interval when component is destroyed
    if (this.refreshInterval) {
      clearInterval(this.refreshInterval)
    }
  },
  methods: {
    async loadConversation() {
      try {
        this.loading = true
        const conversationId = this.$route.params.conversationId
        const data = await getConversation(conversationId)
        this.conversation = data
        // API returns messages in reverse chronological (newest first), sort by timestamp (oldest first)
        const messages = data.messages || []
        this.messages = messages.sort((a, b) => {
          const timeA = new Date(a.sentAt).getTime()
          const timeB = new Date(b.sentAt).getTime()
          return timeA - timeB
        })
        this.$nextTick(() => {
          this.scrollToBottom()
        })
      } catch (error) {
        console.error('Failed to load conversation:', error)
      } finally {
        this.loading = false
      }
    },
    async checkForNewMessages() {
      if (this.loading) return // Don't refresh while initial load is happening
      
      try {
        const conversationId = this.$route.params.conversationId
        const data = await getConversation(conversationId)
        
        // Get current message count for comparison
        const previousCount = this.messages.length
        
        // Sort messages by timestamp (oldest first)
        const messages = data.messages || []
        const sortedMessages = messages.sort((a, b) => {
          const timeA = new Date(a.sentAt).getTime()
          const timeB = new Date(b.sentAt).getTime()
          return timeA - timeB
        })
        
        // Check if there are new messages
        if (sortedMessages.length > previousCount || 
            (sortedMessages.length > 0 && this.messages.length > 0 && 
             sortedMessages[sortedMessages.length - 1].messageId !== this.messages[this.messages.length - 1].messageId)) {
          
          // Replace messages with properly sorted list
          this.messages = sortedMessages
          
          // Update conversation info
          this.conversation = data
          
          // Auto-scroll if user is near bottom
          this.$nextTick(() => {
            const container = this.$refs.messagesContainer
            if (container) {
              const isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 100
              if (isNearBottom) {
                this.scrollToBottom()
              }
            }
          })
        }
      } catch (error) {
        // Silently fail on refresh errors to avoid spamming console
        console.debug('Failed to refresh messages:', error)
      }
    },
    async sendMessage() {
      if (!this.messageText.trim() && !this.selectedFile) {
        return
      }

      try {
        const conversationId = this.$route.params.conversationId
        const msg = await apiSendMessage(conversationId, this.messageText, this.selectedFile)
        this.messages.push(msg)
        this.messageText = ''
        this.selectedFile = null
        this.$nextTick(() => {
          this.scrollToBottom()
        })
      } catch (error) {
        console.error('Failed to send message:', error)
        alert('Failed to send message')
      }
    },
    handleFileSelect(event) {
      const file = event.target.files[0]
      if (file) {
        this.selectedFile = file
        this.sendMessage()
      }
    },
    isSentByMe(message) {
      return message.senderName === this.currentUsername
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
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    },
    getStatusIcon(status) {
      switch (status) {
        case 'sent':
          return '✓'
        case 'received':
          return '✓'
        case 'read':
          return '✓✓'
        default:
          return ''
      }
    },
    getPhotoUrl(photoUrl) {
      if (!photoUrl) return null
      if (photoUrl.startsWith('/')) {
        return __API_URL__ + photoUrl
      }
      return photoUrl
    },
    scrollToBottom() {
      const container = this.$refs.messagesContainer
      if (container) {
        container.scrollTop = container.scrollHeight
      }
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
  background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
}

.chat-header {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border-color);
  padding: 18px 28px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow-md);
  position: sticky;
  top: 0;
  z-index: 10;
}

.chat-title {
  display: flex;
  align-items: center;
  gap: 14px;
}

.chat-title h2 {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
}

.messages-container {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
  background: 
    radial-gradient(circle at 20% 30%, rgba(99, 102, 241, 0.05) 0%, transparent 50%),
    radial-gradient(circle at 80% 70%, rgba(139, 92, 246, 0.05) 0%, transparent 50%);
}

.messages-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 900px;
  margin: 0 auto;
  width: 100%;
}

.message {
  display: flex;
  flex-direction: column;
  max-width: 75%;
  animation: messageSlideIn 0.3s ease-out;
}

@keyframes messageSlideIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.message-sent {
  align-self: flex-end;
}

.message-received {
  align-self: flex-start;
}

.message-sender {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 6px;
  padding-left: 12px;
  font-weight: 500;
}

.message-bubble {
  background: var(--message-received);
  border-radius: var(--radius-lg);
  padding: 12px 18px;
  box-shadow: var(--shadow-sm);
  position: relative;
}

.message-sent .message-bubble {
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-hover) 100%);
  color: white;
  box-shadow: var(--shadow-md);
}

.message-text {
  word-wrap: break-word;
  line-height: 1.5;
  font-size: 15px;
}

.message-photo {
  max-width: 100%;
  max-height: 400px;
  border-radius: var(--radius);
  margin-top: 8px;
  box-shadow: var(--shadow-md);
  object-fit: cover;
}

.message-meta {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  font-size: 12px;
}

.message-sent .message-meta {
  color: rgba(255, 255, 255, 0.9);
}

.message-received .message-meta {
  color: var(--text-muted);
}

.message-reactions {
  display: flex;
  gap: 6px;
  margin-top: 8px;
  padding-left: 12px;
  flex-wrap: wrap;
}

.reaction {
  font-size: 18px;
  cursor: pointer;
  transition: transform 0.2s;
}

.reaction:hover {
  transform: scale(1.2);
}

.message-input-container {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-top: 1px solid var(--border-color);
  padding: 20px 28px;
  display: flex;
  gap: 12px;
  box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.05);
  position: sticky;
  bottom: 0;
}

.message-input {
  flex: 1;
  font-size: 15px;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: none !important;
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
</style>
