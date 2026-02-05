import { getAuthToken } from './auth'

function getHeaders(includeAuth = true) {
  const headers = {
    'Content-Type': 'application/json'
  }

  if (includeAuth) {
    const token = getAuthToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
  }

  return headers
}

export async function searchUsers(query = '') {
  const url = query ? `${__API_URL__}/users?query=${encodeURIComponent(query)}` : `${__API_URL__}/users`
  const response = await fetch(url, {
    headers: getHeaders()
  })

  if (!response.ok) {
    throw new Error('Failed to search users')
  }

  return await response.json()
}

export async function updateUsername(newUsername) {
  const response = await fetch(`${__API_URL__}/users/me/username`, {
    method: 'PUT',
    headers: getHeaders(),
    body: JSON.stringify({ username: newUsername })
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to update username')
  }

  return await response.json()
}

export async function uploadUserPhoto(file) {
  const formData = new FormData()
  formData.append('photo', file)

  const token = getAuthToken()
  const response = await fetch(`${__API_URL__}/users/me/photo`, {
    method: 'PUT',
    headers: {
      'Authorization': `Bearer ${token}`
    },
    body: formData
  })

  if (!response.ok) {
    throw new Error('Failed to upload photo')
  }

  return await response.json()
}

export async function getConversations() {
  const response = await fetch(`${__API_URL__}/conversations`, {
    headers: getHeaders()
  })

  if (!response.ok) {
    throw new Error('Failed to fetch conversations')
  }

  return await response.json()
}

export async function getConversation(conversationId) {
  const response = await fetch(`${__API_URL__}/conversations/${conversationId}`, {
    headers: getHeaders()
  })

  if (!response.ok) {
    throw new Error('Failed to fetch conversation')
  }

  return await response.json()
}

export async function createConversation(userId) {
  const response = await fetch(`${__API_URL__}/conversations`, {
    method: 'POST',
    headers: getHeaders(),
    body: JSON.stringify({ userId })
  })

  if (!response.ok) {
    throw new Error('Failed to create conversation')
  }

  return await response.json()
}

export async function sendMessage(conversationId, content, photo = null, replyTo = null) {
  const formData = new FormData()
  
  if (content) {
    formData.append('content', content)
  }
  if (photo) {
    formData.append('photo', photo)
  }
  if (replyTo) {
    formData.append('replyTo', replyTo)
  }

  const token = getAuthToken()
  const response = await fetch(`${__API_URL__}/conversations/${conversationId}/messages`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`
    },
    body: formData
  })

  if (!response.ok) {
    throw new Error('Failed to send message')
  }

  return await response.json()
}

export async function deleteMessage(messageId) {
  const response = await fetch(`${__API_URL__}/messages/${messageId}`, {
    method: 'DELETE',
    headers: getHeaders()
  })

  if (!response.ok) {
    throw new Error('Failed to delete message')
  }
}

export async function forwardMessage(messageId, targetConversationId) {
  const response = await fetch(`${__API_URL__}/messages/${messageId}/forward`, {
    method: 'POST',
    headers: getHeaders(),
    body: JSON.stringify({ targetConversationId })
  })

  if (!response.ok) {
    throw new Error('Failed to forward message')
  }

  return await response.json()
}

export async function addReaction(messageId, emoticon) {
  const response = await fetch(`${__API_URL__}/messages/${messageId}/comments`, {
    method: 'POST',
    headers: getHeaders(),
    body: JSON.stringify({ emoticon })
  })

  if (!response.ok) {
    throw new Error('Failed to add reaction')
  }

  return await response.json()
}

export async function removeReaction(messageId, reactionId) {
  const response = await fetch(`${__API_URL__}/messages/${messageId}/comments/${reactionId}`, {
    method: 'DELETE',
    headers: getHeaders()
  })

  if (!response.ok) {
    throw new Error('Failed to remove reaction')
  }
}

export async function createGroup(name, memberIds) {
  const response = await fetch(`${__API_URL__}/groups`, {
    method: 'POST',
    headers: getHeaders(),
    body: JSON.stringify({ name, memberIds })
  })

  if (!response.ok) {
    throw new Error('Failed to create group')
  }

  return await response.json()
}

export async function addGroupMember(groupId, userId) {
  const response = await fetch(`${__API_URL__}/groups/${groupId}/members`, {
    method: 'POST',
    headers: getHeaders(),
    body: JSON.stringify({ userId })
  })

  if (!response.ok) {
    throw new Error('Failed to add member')
  }

  return await response.json()
}

export async function leaveGroup(groupId) {
  const response = await fetch(`${__API_URL__}/groups/${groupId}/members/me`, {
    method: 'DELETE',
    headers: getHeaders()
  })

  if (!response.ok) {
    throw new Error('Failed to leave group')
  }
}

export async function updateGroupName(groupId, name) {
  const response = await fetch(`${__API_URL__}/groups/${groupId}/name`, {
    method: 'PUT',
    headers: getHeaders(),
    body: JSON.stringify({ name })
  })

  if (!response.ok) {
    throw new Error('Failed to update group name')
  }

  return await response.json()
}

export async function uploadGroupPhoto(groupId, file) {
  const formData = new FormData()
  formData.append('photo', file)

  const token = getAuthToken()
  const response = await fetch(`${__API_URL__}/groups/${groupId}/photo`, {
    method: 'PUT',
    headers: {
      'Authorization': `Bearer ${token}`
    },
    body: formData
  })

  if (!response.ok) {
    throw new Error('Failed to upload group photo')
  }

  return await response.json()
}
