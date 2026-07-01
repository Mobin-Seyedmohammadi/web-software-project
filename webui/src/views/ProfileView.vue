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
  min-height: 100vh;
  background: linear-gradient(180deg, #f4f5fb 0%, #eef1f8 100%);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  display: flex; flex-direction: column;
}
.profile-header {
  background: #fff; border-bottom: 1px solid #eef0f5;
  padding: 16px 28px; display: flex; justify-content: space-between;
  align-items: center; box-shadow: 0 1px 3px rgba(15, 23, 42, .06), 0 4px 14px rgba(15, 23, 42, .04);
  position: sticky; top: 0; z-index: 10;
}
.profile-header h1 {
  font-size: 22px; font-weight: 800; margin: 0; letter-spacing: -.02em;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text;
}
.profile-body { flex: 1; padding: 40px 16px; display: flex; justify-content: center; }
.profile-card {
  background: #fff; border-radius: 20px; padding: 36px;
  box-shadow: 0 4px 16px rgba(15, 23, 42, .05), 0 16px 48px rgba(15, 23, 42, .07);
  border: 1px solid rgba(15, 23, 42, .03);
  width: 100%; max-width: 500px;
  display: flex; flex-direction: column; gap: 28px;
}

/* Avatar */
.avatar-section { display: flex; flex-direction: column; align-items: center; gap: 10px; }
.avatar-wrap { position: relative; cursor: pointer; border-radius: 50%; }
.avatar-wrap:hover .avatar-overlay { opacity: 1; }
.avatar-lg {
  width: 100px; height: 100px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-weight: 700; color: #fff; font-size: 38px; overflow: hidden;
  box-shadow: 0 0 0 4px #fff, 0 6px 20px rgba(15, 23, 42, .18);
}
.avatar-img { width: 100%; height: 100%; object-fit: cover; }
.avatar-overlay {
  position: absolute; inset: 0; background: rgba(15, 23, 42, .45);
  border-radius: 50%; display: flex; align-items: center; justify-content: center;
  font-size: 24px; opacity: 0; transition: opacity .2s;
}
.avatar-hint { font-size: 12.5px; color: #a1a4b8; font-weight: 500; }

/* Info */
.info-section, .form-section { display: flex; flex-direction: column; gap: 7px; }
.field-label { font-size: 12.5px; font-weight: 700; color: #8b8fa3; text-transform: uppercase; letter-spacing: .06em; }
.current-name { font-size: 25px; font-weight: 800; color: #1e1b3a; letter-spacing: -.01em; }

.input-row { display: flex; gap: 10px; }
.input {
  flex: 1; border: 1.5px solid #e5e7f0; border-radius: 10px;
  padding: 11px 15px; font-size: 15px; outline: none;
  transition: border-color .15s, box-shadow .15s;
}
.input:focus { border-color: #a5a8f5; box-shadow: 0 0 0 3px rgba(99, 102, 241, .1); }

.error-msg { color: #ef4444; font-size: 14px; margin-top: 4px; font-weight: 500; }
.success-msg { color: #10b981; font-size: 14px; margin-top: 4px; font-weight: 500; }

.upload-status { padding: 11px 15px; border-radius: 10px; font-size: 14px; font-weight: 500; }
.upload-status.info { background: #eef0ff; color: #6366f1; }
.upload-status.success { background: #d1fae5; color: #059669; }
.upload-status.error { background: #fee2e2; color: #ef4444; }

/* Buttons */
.btn { padding: 11px 20px; border-radius: 10px; border: none; cursor: pointer; font-size: 14px; font-weight: 600; transition: all .15s; }
.btn-primary {
  background: linear-gradient(135deg, #6366f1 0%, #7c5cf0 100%); color: #fff;
  box-shadow: 0 2px 8px rgba(99, 102, 241, .35);
}
.btn-primary:hover:not(:disabled) { box-shadow: 0 4px 14px rgba(99, 102, 241, .45); transform: translateY(-1px); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; box-shadow: none; }
.btn-icon { background: none; border: none; cursor: pointer; font-size: 20px; color: #767a92; padding: 7px; border-radius: 10px; transition: background .15s; }
.btn-icon:hover { background: #f0f1fb; }
</style>
