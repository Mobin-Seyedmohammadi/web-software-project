<template>
  <div class="toast-container">
    <transition-group name="toast" tag="div">
      <div
        v-for="t in toastState.items"
        :key="t.id"
        :class="['toast', 'toast-' + t.type]"
        @click="dismissToast(t.id)"
      >
        {{ t.message }}
      </div>
    </transition-group>
  </div>
</template>

<script>
import { toastState, dismissToast } from '../services/toast'

export default {
  name: 'ToastContainer',
  data() {
    return { toastState }
  },
  methods: {
    dismissToast
  }
}
</script>

<style scoped>
.toast-container {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.toast {
  pointer-events: auto;
  padding: 12px 18px;
  border-radius: 10px;
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  min-width: 220px;
  max-width: 360px;
  cursor: pointer;
}
.toast-success { background: #10b981; }
.toast-error { background: #ef4444; }
.toast-info { background: #6366f1; }

.toast-enter-active, .toast-leave-active {
  transition: all 0.2s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(30px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(30px);
}
</style>
