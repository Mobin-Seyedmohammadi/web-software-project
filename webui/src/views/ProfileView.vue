<template>
  <div class="profile-view">
    <div class="profile-header">
      <button @click="goBack" class="btn btn-secondary">← Back</button>
      <h1>Profile Settings</h1>
      <div></div>
    </div>

    <div class="profile-container">
      <div class="profile-card">
        <div class="profile-section">
          <h2>Your Information</h2>
          <div class="profile-info">
            <div class="avatar avatar-lg" :style="{ background: getAvatarColor(currentUsername) }">{{ getInitial(currentUsername) }}</div>
            <p class="current-username">{{ currentUsername }}</p>
          </div>
        </div>

        <div class="profile-section">
          <h3>Change Username</h3>
          <div class="form-group">
            <input
              v-model="newUsername"
              type="text"
              class="input"
              placeholder="Enter new username"
              minlength="3"
              maxlength="16"
            />
          </div>
          <button @click="updateUsername" class="btn btn-primary" :disabled="updating">
            {{ updating ? 'Updating...' : 'Update Username' }}
          </button>
        </div>

        <div v-if="errorMessage" class="error-message">
          {{ errorMessage }}
        </div>

        <div v-if="successMessage" class="success-message">
          {{ successMessage }}
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { updateUsername as apiUpdateUsername } from '../services/api'
import { getUsername, setUsername } from '../services/auth'

export default {
  name: 'ProfileView',
  data() {
    return {
      currentUsername: getUsername(),
      newUsername: '',
      errorMessage: '',
      successMessage: '',
      updating: false
    }
  },
  methods: {
    async updateUsername() {
      if (!this.newUsername || this.newUsername.length < 3 || this.newUsername.length > 16) {
        this.errorMessage = 'Username must be between 3 and 16 characters'
        return
      }

      this.errorMessage = ''
      this.successMessage = ''
      this.updating = true

      try {
        await apiUpdateUsername(this.newUsername)
        setUsername(this.newUsername)
        this.currentUsername = this.newUsername
        this.newUsername = ''
        this.successMessage = 'Username updated successfully!'
      } catch (error) {
        this.errorMessage = error.message || 'Failed to update username'
      } finally {
        this.updating = false
      }
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
    goBack() {
      this.$router.push('/')
    }
  }
}
</script>

<style scoped>
.profile-view {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
}

.profile-header {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border-color);
  padding: 20px 32px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow-md);
  position: sticky;
  top: 0;
  z-index: 10;
}

.profile-header h1 {
  font-size: 26px;
  font-weight: 700;
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-light) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.profile-container {
  flex: 1;
  overflow-y: auto;
  padding: 48px 24px;
}

.profile-card {
  max-width: 640px;
  margin: 0 auto;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(10px);
  border-radius: var(--radius-xl);
  padding: 40px;
  box-shadow: var(--shadow-lg);
  border: 1px solid rgba(255, 255, 255, 0.8);
}

.profile-section {
  margin-bottom: 32px;
}

.profile-section:last-child {
  margin-bottom: 0;
}

.profile-section h2 {
  font-size: 20px;
  margin-bottom: 16px;
}

.profile-section h3 {
  font-size: 16px;
  margin-bottom: 12px;
  font-weight: 600;
}

.profile-info {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  padding: 32px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.05) 0%, rgba(139, 92, 246, 0.05) 100%);
  border-radius: var(--radius-lg);
  margin-bottom: 32px;
}

.current-username {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.5px;
}
</style>
