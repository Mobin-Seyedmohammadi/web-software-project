<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <h1>WASAText</h1>
        <p>Connect with friends through messages</p>
      </div>

      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-group">
          <label for="username" class="form-label">Username</label>
          <input
            id="username"
            v-model="username"
            type="text"
            class="input"
            placeholder="Enter your username"
            minlength="3"
            maxlength="16"
            required
          />
        </div>

        <div v-if="errorMessage" class="error-message">
          {{ errorMessage }}
        </div>

        <button type="submit" class="btn btn-primary btn-block" :disabled="loading">
          {{ loading ? 'Logging in...' : 'Log In' }}
        </button>
      </form>

      <div class="login-footer">
        <p>Enter a username to log in or create a new account</p>
      </div>
    </div>
  </div>
</template>

<script>
import { login } from '../services/auth'

export default {
  name: 'LoginView',
  data() {
    return {
      username: '',
      errorMessage: '',
      loading: false
    }
  },
  methods: {
    async handleLogin() {
      this.errorMessage = ''
      this.loading = true

      try {
        await login(this.username)
        this.$router.push('/')
      } catch (error) {
        this.errorMessage = error.message || 'Login failed'
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #6366f1 0%, #7c3aed 50%, #c026d3 100%);
  background-size: 200% 200%;
  animation: gradientShift 15s ease infinite;
  padding: 20px;
  position: relative;
  overflow: hidden;
}

.login-container::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: radial-gradient(circle at 20% 50%, rgba(255, 255, 255, 0.1) 0%, transparent 50%),
              radial-gradient(circle at 80% 80%, rgba(255, 255, 255, 0.1) 0%, transparent 50%);
  pointer-events: none;
}

@keyframes gradientShift {
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}

.login-card {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(20px);
  border-radius: var(--radius-xl);
  padding: 48px;
  max-width: 420px;
  width: 100%;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.3);
  position: relative;
  z-index: 1;
  animation: slideUp 0.5s ease-out;
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.login-header {
  text-align: center;
  margin-bottom: 36px;
}

.login-header h1 {
  font-size: 36px;
  font-weight: 700;
  background: linear-gradient(135deg, var(--primary-color) 0%, #8b5cf6 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 12px;
  letter-spacing: -1px;
}

.login-header p {
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 500;
}

.login-form {
  margin-bottom: 20px;
}

.btn-block {
  width: 100%;
  margin-top: 20px;
}

.login-footer {
  text-align: center;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
}

.login-footer p {
  font-size: 13px;
  color: var(--text-secondary);
}
</style>
