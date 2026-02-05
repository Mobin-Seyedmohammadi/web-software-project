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
            <div class="avatar avatar-lg">{{ getInitial(currentUsername) }}</div>
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

        <div class="profile-section">
          <h3>Update Profile Photo</h3>
          <input
            ref="photoInput"
            type="file"
            accept="image/*"
            style="display: none"
            @change="handlePhotoSelect"
          />
          <button @click="$refs.photoInput.click()" class="btn btn-secondary" :disabled="uploading">
            {{ uploading ? 'Uploading...' : 'Choose Photo' }}
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
import { updateUsername as apiUpdateUsername, uploadUserPhoto } from '../services/api'
import { getUsername, setUsername } from '../services/auth'

export default {
  name: 'ProfileView',
  data() {
    return {
      currentUsername: getUsername(),
      newUsername: '',
      errorMessage: '',
      successMessage: '',
      updating: false,
      uploading: false
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
    async handlePhotoSelect(event) {
      const file = event.target.files[0]
      if (!file) return

      this.errorMessage = ''
      this.successMessage = ''
      this.uploading = true

      try {
        await uploadUserPhoto(file)
        this.successMessage = 'Photo updated successfully!'
      } catch (error) {
        this.errorMessage = error.message || 'Failed to upload photo'
      } finally {
        this.uploading = false
      }
    },
    getInitial(name) {
      return name ? name.charAt(0).toUpperCase() : '?'
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
  background-color: var(--secondary-color);
}

.profile-header {
  background: var(--bg-white);
  border-bottom: 1px solid var(--border-color);
  padding: 16px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow);
}

.profile-header h1 {
  font-size: 24px;
}

.profile-container {
  flex: 1;
  overflow-y: auto;
  padding: 40px 20px;
}

.profile-card {
  max-width: 600px;
  margin: 0 auto;
  background: var(--bg-white);
  border-radius: 16px;
  padding: 32px;
  box-shadow: var(--shadow);
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
  gap: 16px;
  padding: 20px;
}

.current-username {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}
</style>
