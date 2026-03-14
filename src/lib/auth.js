/*
 * User Authentication
 *
 * Manages the user authentication lifecycle via Forumline identity.
 * All authentication goes through Forumline OAuth — no local email/password.
 *
 * It must:
 * - Persist sessions in localStorage
 * - Create a user profile with a generated default avatar on first sign-in
 * - Expose the current user and profile to the rest of the app via a reactive auth store
 */

import { createStore } from '../state.js'
import { api } from './api.js'
import { uploadDefaultAvatar } from './avatars.js'

const STORAGE_KEY = 'forumline-session'

// Auth state store
export const authStore = createStore({
  user: null,     // { id, email, username, avatar, user_metadata }
  profile: null,  // full profile row
  loading: true,
})

let currentSession = null
let expiryTimer = null

// --- Internal helpers ---

function saveSession(session) {
  currentSession = session
  if (session) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(session))
    scheduleExpiryCheck(session)
  } else {
    localStorage.removeItem(STORAGE_KEY)
    // Also clear legacy storage key
    localStorage.removeItem('gotrue-session')
    if (expiryTimer) {
      clearTimeout(expiryTimer)
      expiryTimer = null
    }
  }
}

function scheduleExpiryCheck(session) {
  if (expiryTimer) clearTimeout(expiryTimer)
  const expiresAt = session.expires_at * 1000
  const checkIn = Math.max(expiresAt - Date.now() - 60000, 5000)
  expiryTimer = setTimeout(() => {
    // Session expired — sign out
    if (currentSession?.expires_at * 1000 < Date.now()) {
      saveSession(null)
      authStore.set({ user: null, profile: null, loading: false })
    }
  }, checkIn)
}

async function fetchProfile(userId) {
  return api.getProfile(userId)
}

async function ensureProfile(rawUser) {
  let prof = await fetchProfile(rawUser.id)
  if (prof) {
    authStore.set({ profile: prof })
    return prof
  }

  // Create profile
  const username = rawUser.user_metadata?.username
    || `user_${rawUser.id.slice(0, 8)}`
  const displayName = rawUser.user_metadata?.display_name || username

  try {
    await api.upsertProfile(rawUser.id, { username, display_name: displayName, avatar_url: null })
  } catch (err) {
    console.error('Failed to create profile:', err)
    return null
  }

  // Generate default avatar
  const token = await getAccessToken()
  const avatarUrl = token ? await uploadDefaultAvatar(rawUser.id, 'user', token) : null
  if (avatarUrl) {
    await api.updateProfile(rawUser.id, { avatar_url: avatarUrl })
  }

  prof = await fetchProfile(rawUser.id)
  if (prof) authStore.set({ profile: prof })
  return prof
}

function toAppUser(rawUser, prof) {
  if (!rawUser) return null
  return {
    id: rawUser.id,
    email: rawUser.email || '',
    username: prof?.username,
    avatar: prof?.avatar_url || undefined,
    is_admin: prof?.is_admin || false,
    user_metadata: {
      username: prof?.username || rawUser.user_metadata?.username,
    },
  }
}

// --- Public API ---

export async function getAccessToken() {
  if (!currentSession) return null
  if (currentSession.expires_at * 1000 < Date.now()) {
    // Session expired, no refresh available — sign out
    saveSession(null)
    authStore.set({ user: null, profile: null, loading: false })
    return null
  }
  return currentSession.access_token
}

export async function signOut() {
  saveSession(null)
  authStore.set({ user: null, profile: null, loading: false })
}

async function restoreSessionFromUrl() {
  const hash = window.location.hash
  if (!hash) return false
  const params = new URLSearchParams(hash.substring(1))
  const accessToken = params.get('access_token')
  if (!accessToken) return false

  try {
    const payload = JSON.parse(atob(accessToken.split('.')[1]))
    const userId = payload.sub
    if (!userId) return false

    const session = {
      access_token: accessToken,
      expires_in: (payload.exp - payload.iat) || 86400,
      expires_at: payload.exp || Math.floor(Date.now() / 1000) + 86400,
      user: { id: userId },
    }
    saveSession(session)
    window.history.replaceState({}, '', window.location.pathname)
    return true
  } catch {
    return false
  }
}

// --- Init ---

export async function initAuth() {
  // Restore from localStorage (check both new and legacy keys)
  const stored = localStorage.getItem(STORAGE_KEY) || localStorage.getItem('gotrue-session')
  if (stored) {
    try {
      const session = JSON.parse(stored)
      // Migrate legacy sessions: if it has a refresh_token (GoTrue), discard it
      if (session.refresh_token) {
        localStorage.removeItem('gotrue-session')
        localStorage.removeItem(STORAGE_KEY)
      } else {
        currentSession = session
      }
    } catch {
      localStorage.removeItem(STORAGE_KEY)
      localStorage.removeItem('gotrue-session')
    }
  }
  if (currentSession) scheduleExpiryCheck(currentSession)

  // Check URL hash for OAuth tokens
  await restoreSessionFromUrl()

  // Strip ?forumline_auth=success
  const params = new URLSearchParams(window.location.search)
  if (params.has('forumline_auth')) {
    params.delete('forumline_auth')
    const newUrl = params.toString()
      ? `${window.location.pathname}?${params}`
      : window.location.pathname
    window.history.replaceState({}, '', newUrl)
  }

  // Load profile if session exists
  if (currentSession?.user) {
    try {
      const prof = await ensureProfile(currentSession.user)
      authStore.set({ user: toAppUser(currentSession.user, prof), loading: false })
    } catch {
      authStore.set({ user: toAppUser(currentSession.user, null), loading: false })
    }
  } else {
    authStore.set({ loading: false })
  }
}
