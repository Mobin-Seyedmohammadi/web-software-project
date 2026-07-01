import { reactive } from 'vue'

export const toastState = reactive({ items: [] })

let nextId = 1

export function showToast(message, type = 'info', duration = 3000) {
  const id = nextId++
  toastState.items.push({ id, message, type })
  setTimeout(() => dismissToast(id), duration)
  return id
}

export function dismissToast(id) {
  const idx = toastState.items.findIndex(t => t.id === id)
  if (idx !== -1) toastState.items.splice(idx, 1)
}
