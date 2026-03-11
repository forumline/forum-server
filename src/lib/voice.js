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
import {
  setStoreCallback, setEscalateCallback, sendEscalateSignal,
  joinRoomP2P, leaveRoomP2P, toggleMuteP2P,
  toggleDeafenP2P, handlePeerJoined,
  cleanupP2P,
} from './voice-p2p.js'

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

// Which backend is currently active: 'p2p' | 'livekit' | null
let activeBackend = null
let accessTokenCached = null
const avatarCache = {}
let presenceSSECleanup = null

// LiveKit state (lazy-loaded)
let livekitModule = null
let room = null

async function getLivekit() {
  if (!livekitModule) livekitModule = await import('livekit-client')
  return livekitModule
}

// Wire up P2P store updates
setStoreCallback((update) => {
  if (activeBackend === 'p2p') voiceStore.set(update)
})

// Wire up escalation requests from peers
setEscalateCallback(() => {
  if (activeBackend !== 'p2p') return
  const slug = voiceStore.get().connectedRoomSlug
  const name = voiceStore.get().connectedRoomName
  if (slug) escalateToLiveKit(slug, name)
})

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
      if (row.profile && avatarCache[row.user_id] === undefined) {
        avatarCache[row.user_id] = row.profile.avatar_url
      }
    }
    voiceStore.set({ roomParticipantCounts: counts })

    // If we're in P2P mode, handle peer join/leave from presence changes
    if (activeBackend === 'p2p') {
      const slug = voiceStore.get().connectedRoomSlug
      if (slug) {
        const roomPeers = data.filter(r => r.room_slug === slug)
        const { user } = authStore.get()
        if (user) {
          const currentPeerIDs = new Set(roomPeers.map(p => p.user_id).filter(id => id !== user.id))

          // Notify P2P module of joins
          for (const id of currentPeerIDs) {
            handlePeerJoined(id)
          }

          // Check if we need to escalate to LiveKit (5+ participants)
          if (currentPeerIDs.size + 1 >= 5) {
            escalateToLiveKit(slug, voiceStore.get().connectedRoomName)
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

  // Disconnect from any existing room
  if (voiceStore.get().isConnected) leaveRoom()

  voiceStore.set({ connectError: null, isConnecting: true })

  try {
    const accessToken = await getAccessToken()
    if (!accessToken) { voiceStore.set({ connectError: 'Not authenticated', isConnecting: false }); return }
    accessTokenCached = accessToken

    const displayName = user.username || user.user_metadata?.username || user.email.split('@')[0]

    // Default: use P2P
    activeBackend = 'p2p'
    await joinRoomP2P(slug, name, user.id, displayName, accessToken)

    voiceStore.set({
      isConnected: true, isConnecting: false, isMuted: false, isDeafened: false,
      connectedRoomSlug: slug, connectedRoomName: name,
    })
  } catch (err) {
    activeBackend = null
    voiceStore.set({ connectError: err instanceof Error ? err.message : 'Failed to connect', isConnecting: false })
  }
}

export function leaveRoom() {
  if (activeBackend === 'p2p') {
    leaveRoomP2P()
  } else if (activeBackend === 'livekit') {
    leaveRoomLiveKit()
  }

  deletePresence()
  activeBackend = null
  voiceStore.set({
    isConnected: false, isConnecting: false, participants: [],
    connectedRoomSlug: null, connectedRoomName: null,
    isMuted: false, isDeafened: false, isSpeaking: false,
    isScreenSharing: false, screenShareTrack: null, screenShareParticipant: null, connectError: null,
  })
  accessTokenCached = null
}

export async function toggleMute() {
  const newMuted = !voiceStore.get().isMuted
  if (activeBackend === 'p2p') {
    toggleMuteP2P(newMuted)
  } else if (activeBackend === 'livekit' && room) {
    await room.localParticipant.setMicrophoneEnabled(!newMuted)
  }
  voiceStore.set({ isMuted: newMuted })
}

export function toggleDeafen() {
  const newDeafened = !voiceStore.get().isDeafened
  if (activeBackend === 'p2p') {
    toggleDeafenP2P(newDeafened)
  } else if (activeBackend === 'livekit' && room && livekitModule) {
    const lk = livekitModule
    room.remoteParticipants.forEach(p => {
      p.getTrackPublications().forEach(pub => {
        if (pub.track && pub.track.source === lk.Track.Source.Microphone) {
          if (newDeafened) pub.track.detach()
          else {
            const el = pub.track.attach()
            el.id = `audio-${p.identity}`
            if (!document.getElementById(el.id)) document.body.appendChild(el)
          }
        }
      })
    })
  }
  voiceStore.set({ isDeafened: newDeafened })
  if (newDeafened && !voiceStore.get().isMuted) {
    toggleMute()
  }
}

export async function toggleScreenShare() {
  // Screen sharing requires LiveKit — escalate if on P2P
  if (activeBackend === 'p2p') {
    const slug = voiceStore.get().connectedRoomSlug
    const name = voiceStore.get().connectedRoomName
    if (!slug) return

    voiceStore.set({ isConnecting: true })
    try {
      // Tell all peers to escalate to LiveKit too
      await sendEscalateSignal()
      await escalateToLiveKit(slug, name)
      // Now on LiveKit, start screen share
      if (room) {
        await room.localParticipant.setScreenShareEnabled(true)
      }
    } catch {
      voiceStore.set({ isConnecting: false })
    }
    return
  }

  if (activeBackend === 'livekit' && room) {
    try {
      await room.localParticipant.setScreenShareEnabled(!voiceStore.get().isScreenSharing)
    } catch {}
  }
}

export function getAvatarUrl(identity) {
  return avatarCache[identity] ?? null
}

export function initVoice() {
  fetchVoicePresence()
  presenceSSECleanup = connectSSE('/api/voice-presence/stream', () => fetchVoicePresence(), true)

  window.addEventListener('beforeunload', () => {
    if (activeBackend === 'p2p') leaveRoomP2P()
    if (activeBackend === 'livekit' && room) room.disconnect()
    deletePresence()
  })
}

export function cleanupVoice() {
  if (presenceSSECleanup) presenceSSECleanup()
  cleanupP2P()
  if (room) { room.disconnect(); room = null }
}

// ---- LiveKit backend (for screen sharing and 5+ participant rooms) ----

async function escalateToLiveKit(slug, name) {
  const livekitUrl = getConfig().livekit_url || import.meta.env.VITE_LIVEKIT_URL
  if (!livekitUrl) {
    voiceStore.set({ connectError: 'LiveKit not available for screen sharing' })
    return
  }

  const accessToken = accessTokenCached || await getAccessToken()
  if (!accessToken) return

  const { user } = authStore.get()
  if (!user) return
  const displayName = user.username || user.user_metadata?.username || user.email.split('@')[0]

  // Get LiveKit token
  const resp = await fetch('/api/livekit', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${accessToken}` },
    body: JSON.stringify({ roomName: slug, participantName: displayName }),
  })
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: 'Failed to get LiveKit token' }))
    voiceStore.set({ connectError: err.error || 'Failed to escalate to LiveKit' })
    return
  }

  const { token } = await resp.json()

  // Connect to LiveKit while P2P is still active (overlap for seamless audio)
  const lk = await getLivekit()
  room = new lk.Room()

  room.on(lk.RoomEvent.ParticipantConnected, updateLiveKitParticipants)
  room.on(lk.RoomEvent.ParticipantDisconnected, updateLiveKitParticipants)
  room.on(lk.RoomEvent.TrackMuted, updateLiveKitParticipants)
  room.on(lk.RoomEvent.TrackUnmuted, updateLiveKitParticipants)
  room.on(lk.RoomEvent.ActiveSpeakersChanged, updateLiveKitParticipants)

  room.on(lk.RoomEvent.TrackSubscribed, (track, _pub, participant) => {
    if (track.kind === lk.Track.Kind.Audio) {
      const stream = new MediaStream([track.mediaStreamTrack])
      const ctx = new AudioContext()
      const source = ctx.createMediaStreamSource(stream)
      const merger = ctx.createChannelMerger(2)
      source.connect(merger, 0, 0)
      source.connect(merger, 0, 1)
      const dest = ctx.createMediaStreamDestination()
      merger.connect(dest)
      const el = new Audio()
      el.id = `lk-audio-${track.sid}`
      el.srcObject = dest.stream
      el.autoplay = true
      document.body.appendChild(el)
    }
    if (track.source === lk.Track.Source.ScreenShare && track.kind === lk.Track.Kind.Video) {
      voiceStore.set({
        screenShareTrack: track.mediaStreamTrack,
        screenShareParticipant: { id: participant.identity, name: participant.name || participant.identity },
      })
    }
  })

  room.on(lk.RoomEvent.TrackUnsubscribed, (track) => {
    if (track.source === lk.Track.Source.ScreenShare && track.kind === lk.Track.Kind.Video) {
      voiceStore.set({ screenShareTrack: null, screenShareParticipant: null })
    }
    track.detach().forEach(el => el.remove())
  })

  room.on(lk.RoomEvent.LocalTrackPublished, (pub) => {
    if (pub.track?.source === lk.Track.Source.ScreenShare && pub.track.kind === lk.Track.Kind.Video) {
      voiceStore.set({
        isScreenSharing: true,
        screenShareTrack: pub.track.mediaStreamTrack,
        screenShareParticipant: { id: room.localParticipant.identity, name: room.localParticipant.name || room.localParticipant.identity },
      })
    }
  })

  room.on(lk.RoomEvent.LocalTrackUnpublished, (pub) => {
    if (pub.source === lk.Track.Source.ScreenShare) {
      voiceStore.set({ isScreenSharing: false, screenShareTrack: null, screenShareParticipant: null })
    }
  })

  room.on(lk.RoomEvent.Disconnected, () => {
    voiceStore.set({
      isConnected: false, participants: [], connectedRoomSlug: null, connectedRoomName: null,
      isMuted: false, isDeafened: false, isSpeaking: false,
      isScreenSharing: false, screenShareTrack: null, screenShareParticipant: null,
    })
    room = null
    activeBackend = null
    deletePresence()
  })

  await room.connect(livekitUrl, token)
  await room.localParticipant.setMicrophoneEnabled(true)

  // Now tear down P2P (LiveKit is connected and handling audio)
  leaveRoomP2P()
  activeBackend = 'livekit'

  voiceStore.set({
    isConnected: true, isConnecting: false, isMuted: false, isDeafened: false,
    connectedRoomSlug: slug, connectedRoomName: name,
  })
  updateLiveKitParticipants()
}

function leaveRoomLiveKit() {
  if (room) {
    room.remoteParticipants.forEach(p => {
      p.getTrackPublications().forEach(pub => {
        if (pub.track) pub.track.detach().forEach(el => el.remove())
      })
    })
    room.disconnect()
    room = null
  }
}

function updateLiveKitParticipants() {
  if (!room || !livekitModule) return
  const lk = livekitModule
  const remotes = Array.from(room.remoteParticipants.values()).map(p => {
    const audioTrack = p.getTrackPublications().find(t => t.track?.source === lk.Track.Source.Microphone)
    return {
      id: p.identity,
      name: p.name || p.identity,
      avatar: (p.name || p.identity).charAt(0).toUpperCase(),
      avatarUrl: avatarCache[p.identity] ?? null,
      isSpeaking: p.isSpeaking,
      isMuted: audioTrack?.isMuted ?? true,
    }
  })
  voiceStore.set({
    participants: remotes,
    isSpeaking: room.localParticipant.isSpeaking,
  })

  const uncached = remotes.filter(p => avatarCache[p.id] === undefined)
  if (uncached.length > 0) {
    const ids = uncached.map(p => p.id).join(',')
    fetch(`/api/profiles/batch?ids=${encodeURIComponent(ids)}`)
      .then(r => r.json())
      .then(data => {
        for (const profile of data) avatarCache[profile.id] = profile.avatar_url
        for (const p of uncached) {
          if (avatarCache[p.id] === undefined) avatarCache[p.id] = null
        }
        updateLiveKitParticipants()
      })
      .catch(() => {})
  }
}
