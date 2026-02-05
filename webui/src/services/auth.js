const AUTH_KEY = 'wasatext_auth_token'
const USERNAME_KEY = 'wasatext_username'

export function getAuthToken() {
  return localStorage.getItem(AUTH_KEY)
}

export function setAuthToken(token) {
  localStorage.setItem(AUTH_KEY, token)
}

export function removeAuthToken() {
  localStorage.removeItem(AUTH_KEY)
  localStorage.removeItem(USERNAME_KEY)
}

export function isAuthenticated() {
  return !!getAuthToken()
}

export function getUsername() {
  return localStorage.getItem(USERNAME_KEY)
}

export function setUsername(username) {
  localStorage.setItem(USERNAME_KEY, username)
}

export async function login(username) {
  try {
    const response = await fetch('/session', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ name: username })
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || 'Login failed')
    }

    const data = await response.json()
    setAuthToken(data.identifier)
    setUsername(username)

    return data
  } catch (error) {
    console.error('Login error:', error)
    throw error
  }
}

export function logout() {
  removeAuthToken()
  window.location.href = '/login'
}
