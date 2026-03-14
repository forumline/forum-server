/*
 * Voice Room Orchestrator
 *
 * Lets forum members join voice chat rooms for real-time audio conversations, with automatic infrastructure scaling.
 *
 * It must:
 * - Default to peer-to-peer WebRTC for low-latency, zero-cost audio in small rooms (up to 4 participants)
 * - Automatically escalate to LiveKit server infrastructure when screen sharing is requested or 5+ users join
 * - Track who is in each voice room via presence so the sidebar and room pages show live participant counts
 * - Provide mute, deafen, and screen share controls that work seamlessly across both P2P and LiveKit backends
 * - Clean up all audio connections and presence records when a user leaves or navigates away
 */

import { createStore } from '../state.js'
import { authStore, getAccessToken } from './auth.js'
import { getConfig } from './config.js'
import { connectSSE } from './sse.js'
import { VoiceSession } from '@forumline/shared-voice'

export const voiceStore = createStore({
  isConnected: false,
  isConnecting: false,
  isMuted: false,
  isDeafened: false,
  isSpeaking: false,
  connectedRoomSlug: null,
  connectedRoomName: null,
  connectError: null,
  participants: [],
  isScreenSharing: false,
  screenShareParticipant: null,
  screenShareTrack: null,
  roomParticipantCounts: {},
})

let session = null
let accessTokenCached = null
const avatarCache = {}
let presenceSSECleanup = null

function createSession() {
  const livekitUrl = getConfig().livekit_url || import.meta.env.VITE_LIVEKIT_URL

  const config = {
    mode: 'auto',
    escalateAt: 5,
    getAuthToken: () => getAccessToken(),
  }

  if (livekitUrl) {
    config.sfu = {
      url: livekitUrl,
      getRoomToken: async (roomName, participantName) => {
        const token = await getAccessToken()
        const resp = await fetch('/api/livekit', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
          body: JSON.stringify({ roomName, participantName }),
        })
        if (!resp.ok) throw new Error('Failed to get LiveKit token')
        const data = await resp.json()
        return data.token
      },
    }
  }

  return new VoiceSession(config)
}

function syncState(state) {
  voiceStore.set({
    isConnected: state.status === 'connected' || state.status === 'active',
    isConnecting: state.status === 'connecting',
    isMuted: state.muted,
    isDeafened: state.deafened,
    isSpeaking: state.speaking,
    isScreenSharing: state.screenSharing,
    screenShareTrack: state.screenShareTrack,
    screenShareParticipant: state.screenSharePeerId
      ? { id: state.screenSharePeerId, name: state.screenSharePeerId }
      : null,
    connectError: state.error,
    participants: state.peers.map(p => ({
      id: p.id,
      name: avatarCache[p.id]?.name || p.id.slice(0, 8),
      avatar: (avatarCache[p.id]?.name || p.id).charAt(0).toUpperCase(),
      avatarUrl: avatarCache[p.id]?.avatarUrl ?? null,
      isSpeaking: p.speaking,
      isMuted: p.muted,
    })),
  })

  // Resolve names for peers we haven't cached
  const unresolved = state.peers.filter(p => !avatarCache[p.id])
  if (unresolved.length > 0) {
    const ids = unresolved.map(p => p.id).join(',')
    fetch(`/api/profiles/batch?ids=${encodeURIComponent(ids)}`)
      .then(r => r.json())
      .then(data => {
        for (const profile of data) {
          avatarCache[profile.id] = {
            name: profile.display_name || profile.username,
            avatarUrl: profile.avatar_url,
          }
        }
        // Re-sync with resolved names
        if (session) syncState(session.state)
      })
      .catch(() => {})
  }
}

function deletePresence() {
  const token = accessTokenCached
  if (token) {
    fetch('/api/voice-presence', {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${token}` },
      keepalive: true,
    }).catch(() => {})
  }
}

export async function fetchVoicePresence() {
  try {
    const res = await fetch('/api/voice-presence')
    if (!res.ok) return
    const data = await res.json()

    const counts = {}
    for (const row of data) {
      if (!counts[row.room_slug]) counts[row.room_slug] = { count: 0, names: [], identities: [] }
      counts[row.room_slug].count++
      counts[row.room_slug].identities.push(row.user_id)
      const name = row.profile?.display_name || row.profile?.username || row.user_id.slice(0, 8)
      counts[row.room_slug].names.push(name)
      if (row.profile && !avatarCache[row.user_id]) {
        avatarCache[row.user_id] = {
          name: row.profile.display_name || row.profile.username,
          avatarUrl: row.profile.avatar_url,
        }
      }
    }
    voiceStore.set({ roomParticipantCounts: counts })

    // Notify session of peer joins from presence
    if (session && voiceStore.get().isConnected) {
      const slug = voiceStore.get().connectedRoomSlug
      if (slug) {
        const { user } = authStore.get()
        if (user) {
          const roomPeers = data.filter(r => r.room_slug === slug && r.user_id !== user.id)
          for (const peer of roomPeers) {
            session.addPeer(peer.user_id)
          }
        }
      }
    }
  } catch {}
}

export async function joinRoom(slug, name) {
  const { user } = authStore.get()
  if (!user) return

  if (voiceStore.get().connectedRoomSlug === slug && voiceStore.get().isConnected) return

  if (voiceStore.get().isConnected) leaveRoom()

  voiceStore.set({ connectError: null, isConnecting: true })

  try {
    const accessToken = await getAccessToken()
    if (!accessToken) { voiceStore.set({ connectError: 'Not authenticated', isConnecting: false }); return }
    accessTokenCached = accessToken

    const displayName = user.username || user.user_metadata?.username || user.email.split('@')[0]

    session = createSession()
    session.onStateChange(syncState)
    await session.connect(user.id, displayName, slug)

    // Write presence
    await fetch('/api/voice-presence', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${accessToken}` },
      body: JSON.stringify({ room_slug: slug }),
    }).catch(() => {})

    voiceStore.set({
      isConnected: true, isConnecting: false, isMuted: false, isDeafened: false,
      connectedRoomSlug: slug, connectedRoomName: name,
    })

    // Connect to existing peers from presence
    await fetchVoicePresence()
  } catch (err) {
    session?.destroy()
    session = null
    voiceStore.set({ connectError: err instanceof Error ? err.message : 'Failed to connect', isConnecting: false })
  }
}

export function leaveRoom() {
  if (session) {
    session.destroy()
    session = null
  }

  deletePresence()
  voiceStore.set({
    isConnected: false, isConnecting: false, participants: [],
    connectedRoomSlug: null, connectedRoomName: null,
    isMuted: false, isDeafened: false, isSpeaking: false,
    isScreenSharing: false, screenShareTrack: null, screenShareParticipant: null, connectError: null,
  })
  accessTokenCached = null
}

export async function toggleMute() {
  if (!session) return
  const newMuted = !voiceStore.get().isMuted
  session.setMuted(newMuted)
}

export function toggleDeafen() {
  if (!session) return
  const newDeafened = !voiceStore.get().isDeafened
  session.setDeafened(newDeafened)
}

export async function toggleScreenShare() {
  if (!session) return
  const current = voiceStore.get().isScreenSharing
  await session.setScreenShareEnabled(!current)
}

export function getAvatarUrl(identity) {
  return avatarCache[identity]?.avatarUrl ?? null
}

export function initVoice() {
  fetchVoicePresence()
  presenceSSECleanup = connectSSE('/api/voice-presence/stream', () => fetchVoicePresence(), true)

  window.addEventListener('beforeunload', () => {
    if (session) session.destroy()
    deletePresence()
  })
}

export function cleanupVoice() {
  if (presenceSSECleanup) presenceSSECleanup()
  if (session) { session.destroy(); session = null }
}
