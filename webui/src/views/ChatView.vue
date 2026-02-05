<template>
  <div class="chat-view">
    <div class="chat-header">
      <button @click="goBack" class="btn btn-secondary">← Back</button>
      <div v-if="conversation" class="chat-title">
        <div class="avatar avatar-sm">{{ getInitial(conversation.displayName) }}</div>
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
            <img v-if="msg.photoUrl" :src="msg.photoUrl" class="message-photo" />
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
      <button @click="$refs.fileInput.click()" class="btn btn-secondary">📷</button>
      <button @click="sendMessage" class="btn btn-primary">Send</button>
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
      currentUsername: getUsername()
    }
  },
  mounted() {
    this.loadConversation()
  },
  methods: {
    async loadConversation() {
      try {
        this.loading = true
        const conversationId = this.$route.params.conversationId
        const data = await getConversation(conversationId)
        this.conversation = data
        this.messages = (data.messages || []).reverse()
        this.$nextTick(() => {
          this.scrollToBottom()
        })
      } catch (error) {
        console.error('Failed to load conversation:', error)
      } finally {
        this.loading = false
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
  background-color: var(--secondary-color);
}

.chat-header {
  background: var(--bg-white);
  border-bottom: 1px solid var(--border-color);
  padding: 16px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow);
}

.chat-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.chat-title h2 {
  font-size: 18px;
  font-weight: 600;
}

.messages-container {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  display: flex;
  flex-direction: column;
}

.messages-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.message {
  display: flex;
  flex-direction: column;
  max-width: 70%;
}

.message-sent {
  align-self: flex-end;
}

.message-received {
  align-self: flex-start;
}

.message-sender {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 4px;
  padding-left: 8px;
}

.message-bubble {
  background: var(--message-received);
  border-radius: 18px;
  padding: 10px 16px;
}

.message-sent .message-bubble {
  background: var(--message-sent);
  color: white;
}

.message-text {
  word-wrap: break-word;
}

.message-photo {
  max-width: 100%;
  border-radius: 8px;
  margin-top: 4px;
}

.message-meta {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  font-size: 11px;
}

.message-sent .message-meta {
  color: rgba(255, 255, 255, 0.8);
}

.message-received .message-meta {
  color: var(--text-secondary);
}

.message-reactions {
  display: flex;
  gap: 4px;
  margin-top: 4px;
  padding-left: 8px;
}

.reaction {
  font-size: 16px;
}

.message-input-container {
  background: var(--bg-white);
  border-top: 1px solid var(--border-color);
  padding: 16px 24px;
  display: flex;
  gap: 12px;
  box-shadow: var(--shadow);
}

.message-input {
  flex: 1;
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
</style>
