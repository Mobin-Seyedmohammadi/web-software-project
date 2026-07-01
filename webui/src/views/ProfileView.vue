<template>
  <div class="profile-view">
    <div class="profile-header">
      <button @click="$router.push('/')" class="btn btn-icon">&#8592; Back</button>
      <h1>Profile</h1>
      <div style="width:60px"></div>
    </div>

    <div class="profile-body">
      <div class="profile-card">
        <!-- Avatar / photo upload -->
        <div class="avatar-section">
          <div class="avatar-wrap" @click="$refs.photoInput.click()" title="Click to change photo">
            <div class="avatar-lg" :style="avatarStyle(currentUsername)">
              <img v-if="currentPhotoUrl" :src="resolveUrl(currentPhotoUrl)" class="avatar-img" />
              <span v-else>{{ initial(currentUsername) }}</span>
            </div>
            <div class="avatar-overlay">&#128247;</div>
          </div>
          <input ref="photoInput" type="file" accept="image/*" style="display:none" @change="uploadPhoto" />
          <p class="avatar-hint">Click photo to change</p>
        </div>

        <!-- Username display -->
        <div class="info-section">
          <label class="field-label">Current Username</label>
          <div class="current-name">{{ currentUsername }}</div>
        </div>

        <!-- Change username -->
        <div class="form-section">
          <label class="field-label">New Username</label>
          <div class="input-row">
            <input
              v-model="newUsername"
              class="input"
              type="text"
              placeholder="Enter new username"
              minlength="3"
              maxlength="16"
              @keydown.enter="updateUsername"
            />
            <button class="btn btn-primary" :disabled="updating || !newUsername.trim()" @click="updateUsername">
              {{ updating ? 'Saving...' : 'Update' }}
            </button>
          </div>
          <div v-if="errorMsg" class="error-msg">{{ errorMsg }}</div>
          <div v-if="successMsg" class="success-msg">{{ successMsg }}</div>
        </div>

        <!-- Upload status -->
        <div v-if="uploadStatus" class="upload-status" :class="uploadStatus.type">
          {{ uploadStatus.text }}
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { updateUsername as apiUpdateUsername, uploadUserPhoto, searchUsers } from '../services/api'
import { getUsername, setUsername } from '../services/auth'
import { API_URL } from '../services/config'

export default {
  name: 'ProfileView',
  data() {
    return {
      currentUsername: getUsername() || '',
      currentPhotoUrl: null,
      newUsername: '',
      errorMsg: '',
      successMsg: '',
      updating: false,
      uploadStatus: null,
    }
  },
  mounted() {
    this.loadProfile()
  },
  methods: {
    async loadProfile() {
      try {
        const data = await searchUsers(this.currentUsername)
        const me = (data.users || []).find(u => u.username === this.currentUsername)
        if (me) this.currentPhotoUrl = me.photoUrl || null
      } catch (e) { /* silent */ }
    },
    async updateUsername() {
      const u = this.newUsername.trim()
      if (!u) return
      if (u.length < 3 || u.length > 16) {
        this.errorMsg = 'Username must be 3–16 characters'
        return
      }
      this.errorMsg = ''
      this.successMsg = ''
      this.updating = true
      try {
        const result = await apiUpdateUsername(u)
        setUsername(result.username || u)
        this.currentUsername = result.username || u
        this.currentPhotoUrl = result.photoUrl || this.currentPhotoUrl
        this.newUsername = ''
        this.successMsg = 'Username updated!'
      } catch (e) {
        this.errorMsg = e.message || 'Failed to update username'
      } finally {
        this.updating = false
      }
    },
    async uploadPhoto(e) {
      const f = e.target.files[0]
      if (!f) return
      this.uploadStatus = { type: 'info', text: 'Uploading...' }
      try {
        const result = await uploadUserPhoto(f)
        this.currentPhotoUrl = result.photoUrl || null
        this.uploadStatus = { type: 'success', text: 'Photo updated!' }
      } catch (err) {
        this.uploadStatus = { type: 'error', text: 'Failed to upload photo: ' + (err.message || '') }
      }
      e.target.value = ''
      setTimeout(() => { this.uploadStatus = null }, 3000)
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
    }
  }
}
</script>

<style scoped>
.profile-view {
  min-height: 100vh; background: #f0f2f5;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  display: flex; flex-direction: column;
}
.profile-header {
  background: #fff; border-bottom: 1px solid #e5e7eb;
  padding: 14px 24px; display: flex; justify-content: space-between;
  align-items: center; box-shadow: 0 1px 3px rgba(0,0,0,.08);
  position: sticky; top: 0; z-index: 10;
}
.profile-header h1 { font-size: 22px; font-weight: 700; color: #6366f1; margin: 0; }
.profile-body { flex: 1; padding: 32px 16px; display: flex; justify-content: center; }
.profile-card {
  background: #fff; border-radius: 16px; padding: 32px;
  box-shadow: 0 4px 16px rgba(0,0,0,.08); width: 100%; max-width: 500px;
  display: flex; flex-direction: column; gap: 28px;
}

/* Avatar */
.avatar-section { display: flex; flex-direction: column; align-items: center; gap: 8px; }
.avatar-wrap { position: relative; cursor: pointer; border-radius: 50%; }
.avatar-wrap:hover .avatar-overlay { opacity: 1; }
.avatar-lg {
  width: 96px; height: 96px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-weight: 700; color: #fff; font-size: 36px; overflow: hidden;
}
.avatar-img { width: 100%; height: 100%; object-fit: cover; }
.avatar-overlay {
  position: absolute; inset: 0; background: rgba(0,0,0,.4);
  border-radius: 50%; display: flex; align-items: center; justify-content: center;
  font-size: 24px; opacity: 0; transition: opacity .2s;
}
.avatar-hint { font-size: 12px; color: #9ca3af; }

/* Info */
.info-section, .form-section { display: flex; flex-direction: column; gap: 6px; }
.field-label { font-size: 13px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: .05em; }
.current-name { font-size: 24px; font-weight: 700; color: #111827; }

.input-row { display: flex; gap: 10px; }
.input {
  flex: 1; border: 1px solid #d1d5db; border-radius: 8px;
  padding: 10px 14px; font-size: 15px; outline: none;
}
.input:focus { border-color: #6366f1; }

.error-msg { color: #ef4444; font-size: 14px; margin-top: 4px; }
.success-msg { color: #10b981; font-size: 14px; margin-top: 4px; }

.upload-status { padding: 10px 14px; border-radius: 8px; font-size: 14px; }
.upload-status.info { background: #eef2ff; color: #6366f1; }
.upload-status.success { background: #d1fae5; color: #059669; }
.upload-status.error { background: #fee2e2; color: #ef4444; }

/* Buttons */
.btn { padding: 10px 18px; border-radius: 8px; border: none; cursor: pointer; font-size: 14px; font-weight: 500; transition: all .15s; }
.btn-primary { background: #6366f1; color: #fff; }
.btn-primary:hover:not(:disabled) { background: #4f46e5; }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-icon { background: none; border: none; cursor: pointer; font-size: 20px; color: #6b7280; padding: 6px; border-radius: 8px; }
.btn-icon:hover { background: #f3f4f6; }
</style>
