<template>
  <div class="new-group-view">
    <h1>New Group</h1>
    <p>This view is redirected from home</p>
  </div>
</template>

<script>
export default {
  name: 'NewGroupView',
  mounted() {
    this.$router.push('/')
  }
}
</script>

<style scoped>
.new-group-view {
  padding: 20px;
}
</style>
