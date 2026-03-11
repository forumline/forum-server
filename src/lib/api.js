/*
 * Forum API Client
 *
 * Provides all data access methods the forum UI needs to read and write content on the Go backend.
 *
 * It must:
 * - Expose named methods for every forum operation (threads, posts, chat, bookmarks, voice, admin, notifications)
 * - Automatically attach the user's auth token to requests that require authentication
 * - Surface server-side error messages to callers so the UI can display meaningful feedback
 */

import { getAccessToken } from './auth.js'

async function headers() {
  const token = await getAccessToken()
  const h = { 'Content-Type': 'application/json' }
  if (token) h['Authorization'] = `Bearer ${token}`
  return h
}

async function get(url, auth = false) {
  const h = auth ? await headers() : { 'Content-Type': 'application/json' }
  const res = await fetch(url, { headers: h })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `GET ${url} failed: ${res.status}`)
  }
  return res.json()
}

async function post(url, body) {
  const res = await fetch(url, {
    method: 'POST',
    headers: await headers(),
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || `POST ${url} failed: ${res.status}`)
  }
  return res.json()
}

async function put(url, body) {
  const res = await fetch(url, {
    method: 'PUT',
    headers: await headers(),
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || `PUT ${url} failed: ${res.status}`)
  }
}

async function patch(url, body) {
  const res = await fetch(url, {
    method: 'PATCH',
    headers: await headers(),
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || `PATCH ${url} failed: ${res.status}`)
  }
}

async function del(url) {
  const res = await fetch(url, {
    method: 'DELETE',
    headers: await headers(),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || `DELETE ${url} failed: ${res.status}`)
  }
}

export const api = {
  // Reads
  getCategories: () => get('/api/categories'),
  getChannels: () => get('/api/channels'),
  getVoiceRooms: () => get('/api/voice-rooms'),
  getThreads: (limit = 20) => get(`/api/threads?limit=${limit}`),
  getThreadsByCategory: (slug) => get(`/api/categories/${encodeURIComponent(slug)}/threads`),
  getThread: (id) => get(`/api/threads/${encodeURIComponent(id)}`).catch(() => null),
  getPosts: (threadId) => get(`/api/threads/${encodeURIComponent(threadId)}/posts`),
  getCategory: (slug) => get(`/api/categories/${encodeURIComponent(slug)}`).catch(() => null),
  getProfile: (id) => get(`/api/profiles/${encodeURIComponent(id)}`).catch(() => null),
  getProfileByUsername: (username) => get(`/api/profiles/by-username/${encodeURIComponent(username)}`).catch(() => null),
  getChatMessages: (slug) => get(`/api/channels/${encodeURIComponent(slug)}/messages`),
  getBookmarksWithMeta: () => get('/api/bookmarks', true),
  isBookmarked: (threadId) => get(`/api/bookmarks/${encodeURIComponent(threadId)}/status`, true).then(r => r.bookmarked),
  getUserThreads: (userId) => get(`/api/users/${encodeURIComponent(userId)}/threads`),
  getUserPosts: (userId) => get(`/api/users/${encodeURIComponent(userId)}/posts`),
  searchThreads: (q) => q.trim() ? get(`/api/search/threads?q=${encodeURIComponent(q)}`) : Promise.resolve([]),
  searchPosts: (q) => q.trim() ? get(`/api/search/posts?q=${encodeURIComponent(q)}`) : Promise.resolve([]),
  getAdminStats: () => get('/api/admin/stats', true),
  getAdminUsers: () => get('/api/admin/users', true),
  getNotifications: () => get('/api/notifications', true),

  // Writes
  createThread: (input) => post('/api/threads', input),
  updateThread: (id, updates) => patch(`/api/threads/${encodeURIComponent(id)}`, updates),
  createPost: (input) => post('/api/posts', input),
  sendChatMessage: (input) => post(`/api/channels/_by-id/${encodeURIComponent(input.channel_id)}/messages`, { content: input.content }),
  addBookmark: (threadId) => post('/api/bookmarks', { thread_id: threadId }),
  removeBookmark: (threadId) => del(`/api/bookmarks/${encodeURIComponent(threadId)}`),
  removeBookmarkById: (id) => del(`/api/bookmarks/by-id/${encodeURIComponent(id)}`),
  markNotificationRead: (id) => post('/api/forumline/notifications/read', { id }),
  markAllNotificationsRead: () => post('/api/notifications/read-all', {}),
  upsertProfile: (userId, data) => put(`/api/profiles/${encodeURIComponent(userId)}`, data),
  updateProfile: (userId, updates) => put(`/api/profiles/${encodeURIComponent(userId)}`, updates),
  setVoicePresence: (roomSlug) => put('/api/voice-presence', { room_slug: roomSlug }),
  clearVoicePresence: () => del('/api/voice-presence'),

  // Channel follows
  getChannelFollows: () => get('/api/channel-follows', true).catch(() => []),
  followCategory: (categoryId) => post('/api/channel-follows', { category_id: categoryId }),
  unfollowCategory: (categoryId) => del(`/api/channel-follows?category_id=${encodeURIComponent(categoryId)}`),

  // Notification preferences
  getNotificationPreferences: () => get('/api/notification-preferences', true),
  updateNotificationPreference: (category, enabled) => put('/api/notification-preferences', { category, enabled }),
}
