<template>
  <div class="new-chat-view">
    <h1>New Chat</h1>
    <p>This view is redirected from home</p>
  </div>
</template>

<script>
export default {
  name: 'NewChatView',
  mounted() {
    this.$router.push('/')
  }
}
</script>

<style scoped>
.new-chat-view {
  padding: 20px;
}
</style>
