/*
 * Peer-to-Peer Voice Engine
 *
 * Powers direct browser-to-browser voice chat so small rooms work instantly with no server infrastructure cost.
 *
 * It must:
 * - Establish a full mesh of WebRTC connections between all participants in a room (up to 4 peers)
 * - Use the forum's SSE signaling channel to exchange WebRTC offers, answers, and ICE candidates
 * - Detect who is speaking in real time (both local and remote) so the UI can show speaking indicators
 * - Support mute and deafen controls that immediately affect the local audio stream and remote playback
 * - Signal all peers to escalate to LiveKit when a participant requests screen sharing or the room grows too large
 */

import { getAccessToken } from './auth.js'
import { connectSSE } from './sse.js'

const ICE_SERVERS = [
  { urls: 'stun:stun.l.google.com:19302' },
  { urls: 'stun:stun1.l.google.com:19302' },
]

// State managed by this module, written to the voiceStore by the dispatcher
let peers = new Map()          // userId -> { pc: RTCPeerConnection, audioEl: HTMLAudioElement }
let localStream = null
let signalCleanup = null
let currentRoomSlug = null
let currentUserID = null
let speakingInterval = null
let localAudioContext = null
let localAnalyser = null
let onStoreUpdate = null       // callback to update voiceStore
let onEscalateRequest = null   // callback when a peer requests escalation to LiveKit

export function setStoreCallback(cb) {
  onStoreUpdate = cb
}

export function setEscalateCallback(cb) {
  onEscalateRequest = cb
}

// Send "escalate" signal to all connected peers
export async function sendEscalateSignal() {
  const token = await getAccessToken()
  if (!token || !currentRoomSlug) return

  for (const [peerID] of peers) {
    await sendSignal(token, {
      target_user_id: peerID,
      type: 'escalate',
      room_slug: currentRoomSlug,
      payload: {},
    })
  }
}

export async function joinRoomP2P(slug, name, userID, displayName, accessToken) {
  currentRoomSlug = slug
  currentUserID = userID

  try {
    localStream = await navigator.mediaDevices.getUserMedia({ audio: true })
  } catch (err) {
    throw new Error(err.name === 'NotAllowedError' ? 'Permission denied' : 'Failed to access microphone')
  }

  // Start speaking detection for local user
  startLocalSpeakingDetection()

  // Open signal SSE stream
  signalCleanup = connectSSE(
    `/api/voice-signal/stream`,
    handleIncomingSignal,
    true
  )

  // Fetch current presence to connect to existing participants
  const presRes = await fetch('/api/voice-presence')
  if (presRes.ok) {
    const presData = await presRes.json()
    const roomPeers = presData.filter(p => p.room_slug === slug && p.user_id !== userID)

    for (const peer of roomPeers) {
      // Lexicographic tiebreaker: lower ID sends the offer
      if (userID < peer.user_id) {
        await createAndSendOffer(peer.user_id, slug, accessToken)
      }
      // If we have the higher ID, we wait for their offer via SSE
    }
  }

  // Write our presence
  await fetch('/api/voice-presence', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${accessToken}` },
    body: JSON.stringify({ room_slug: slug }),
  }).catch(() => {})

  updatePeerList()
}

export function leaveRoomP2P() {
  // Close all peer connections
  for (const [, peer] of peers) {
    peer.pc.close()
    if (peer.audioEl) peer.audioEl.remove()
  }
  peers.clear()

  // Stop local media
  if (localStream) {
    localStream.getTracks().forEach(t => t.stop())
    localStream = null
  }

  // Stop speaking detection
  if (speakingInterval) {
    clearInterval(speakingInterval)
    speakingInterval = null
  }
  if (localAudioContext) {
    localAudioContext.close().catch(() => {})
    localAudioContext = null
    localAnalyser = null
  }

  // Close signal stream
  if (signalCleanup) {
    signalCleanup()
    signalCleanup = null
  }

  currentRoomSlug = null
  currentUserID = null
}

export function toggleMuteP2P(isMuted) {
  if (!localStream) return
  localStream.getAudioTracks().forEach(t => { t.enabled = !isMuted })
}

export function toggleDeafenP2P(isDeafened) {
  for (const [, peer] of peers) {
    if (peer.audioEl) peer.audioEl.muted = isDeafened
  }
}

export function handlePeerJoined(peerUserID) {
  if (!currentRoomSlug || !currentUserID || peerUserID === currentUserID) return
  if (peers.has(peerUserID)) return

  // Lexicographic tiebreaker: lower ID sends the offer
  if (currentUserID < peerUserID) {
    getAccessToken().then(token => {
      if (token) createAndSendOffer(peerUserID, currentRoomSlug, token)
    })
  }
}

export function handlePeerLeft(peerUserID) {
  const peer = peers.get(peerUserID)
  if (peer) {
    peer.pc.close()
    if (peer.audioEl) peer.audioEl.remove()
    peers.delete(peerUserID)
    updatePeerList()
  }
}

export function isP2PActive() {
  return currentRoomSlug !== null
}

export function getP2PPeerCount() {
  return peers.size
}

// ---- Internal ----

async function createAndSendOffer(targetUserID, roomSlug, accessToken) {
  const pc = createPeerConnection(targetUserID)
  const offer = await pc.createOffer()
  await pc.setLocalDescription(offer)

  await sendSignal(accessToken, {
    target_user_id: targetUserID,
    type: 'offer',
    room_slug: roomSlug,
    payload: { sdp: pc.localDescription.sdp, type: pc.localDescription.type },
  })
}

function createPeerConnection(remoteUserID) {
  const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS })

  // Add local audio tracks
  if (localStream) {
    localStream.getTracks().forEach(t => pc.addTrack(t, localStream))
  }

  // Handle incoming audio
  pc.ontrack = (event) => {
    const stream = event.streams[0] || new MediaStream([event.track])
    let audioEl = peers.get(remoteUserID)?.audioEl
    if (!audioEl) {
      audioEl = new Audio()
      audioEl.id = `p2p-audio-${remoteUserID}`
      audioEl.autoplay = true
      document.body.appendChild(audioEl)
    }
    audioEl.srcObject = stream

    // Start remote speaking detection
    startRemoteSpeakingDetection(remoteUserID, stream)
  }

  // ICE candidates
  pc.onicecandidate = (event) => {
    if (event.candidate) {
      getAccessToken().then(token => {
        if (token) {
          sendSignal(token, {
            target_user_id: remoteUserID,
            type: 'ice-candidate',
            room_slug: currentRoomSlug,
            payload: event.candidate.toJSON(),
          })
        }
      })
    }
  }

  // Connection state changes
  pc.onconnectionstatechange = () => {
    if (pc.connectionState === 'failed' || pc.connectionState === 'closed') {
      const peer = peers.get(remoteUserID)
      if (peer && peer.pc === pc) {
        peer.pc.close()
        if (peer.audioEl) peer.audioEl.remove()
        peers.delete(remoteUserID)
        updatePeerList()
      }
    } else if (pc.connectionState === 'connected') {
      updatePeerList()
    }
  }

  const audioEl = peers.get(remoteUserID)?.audioEl || null
  peers.set(remoteUserID, { pc, audioEl, isSpeaking: false, name: null, avatarUrl: null })
  return pc
}

async function handleIncomingSignal(signal) {
  const { sender_user_id, type, payload, room_slug } = signal

  if (room_slug !== currentRoomSlug) return

  if (type === 'offer') {
    let peer = peers.get(sender_user_id)
    let pc
    if (peer) {
      // Re-negotiation: reuse existing connection
      pc = peer.pc
    } else {
      pc = createPeerConnection(sender_user_id)
    }

    await pc.setRemoteDescription(new RTCSessionDescription(payload))
    const answer = await pc.createAnswer()
    await pc.setLocalDescription(answer)

    const token = await getAccessToken()
    if (token) {
      await sendSignal(token, {
        target_user_id: sender_user_id,
        type: 'answer',
        room_slug: currentRoomSlug,
        payload: { sdp: pc.localDescription.sdp, type: pc.localDescription.type },
      })
    }
  } else if (type === 'answer') {
    const peer = peers.get(sender_user_id)
    if (peer) {
      await peer.pc.setRemoteDescription(new RTCSessionDescription(payload))
    }
  } else if (type === 'ice-candidate') {
    const peer = peers.get(sender_user_id)
    if (peer) {
      await peer.pc.addIceCandidate(new RTCIceCandidate(payload)).catch(() => {})
    }
  } else if (type === 'escalate') {
    // A peer is requesting we switch to LiveKit
    if (onEscalateRequest) onEscalateRequest()
  }
}

async function sendSignal(accessToken, signal) {
  await fetch('/api/voice-signal', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${accessToken}`,
    },
    body: JSON.stringify(signal),
  }).catch(() => {})
}

function updatePeerList() {
  if (!onStoreUpdate) return

  const participants = []
  for (const [id, peer] of peers) {
    if (peer.pc.connectionState === 'connected' || peer.pc.connectionState === 'connecting') {
      participants.push({
        id,
        name: peer.name || id.slice(0, 8),
        avatar: (peer.name || id).charAt(0).toUpperCase(),
        avatarUrl: peer.avatarUrl ?? null,
        isSpeaking: peer.isSpeaking || false,
        isMuted: false, // We can't know remote mute state in P2P without data channels
      })
    }
  }

  // Fetch profile data for peers we haven't resolved yet
  const unresolved = participants.filter(p => !peers.get(p.id)?.name)
  if (unresolved.length > 0) {
    const ids = unresolved.map(p => p.id).join(',')
    fetch(`/api/profiles/batch?ids=${encodeURIComponent(ids)}`)
      .then(r => r.json())
      .then(data => {
        for (const profile of data) {
          const peer = peers.get(profile.id)
          if (peer) {
            peer.name = profile.display_name || profile.username
            peer.avatarUrl = profile.avatar_url
          }
        }
        // Re-trigger update with resolved names
        if (onStoreUpdate) {
          const resolved = []
          for (const [id, pr] of peers) {
            if (pr.pc.connectionState === 'connected' || pr.pc.connectionState === 'connecting') {
              resolved.push({
                id,
                name: pr.name || id.slice(0, 8),
                avatar: (pr.name || id).charAt(0).toUpperCase(),
                avatarUrl: pr.avatarUrl ?? null,
                isSpeaking: pr.isSpeaking || false,
                isMuted: false,
              })
            }
          }
          onStoreUpdate({ participants: resolved })
        }
      })
      .catch(() => {})
  }

  onStoreUpdate({ participants })
}

// Speaking detection using Web Audio API AnalyserNode
function startLocalSpeakingDetection() {
  if (!localStream || speakingInterval) return

  try {
    localAudioContext = new AudioContext()
    localAnalyser = localAudioContext.createAnalyser()
    localAnalyser.fftSize = 512
    const source = localAudioContext.createMediaStreamSource(localStream)
    source.connect(localAnalyser)

    const dataArray = new Uint8Array(localAnalyser.frequencyBinCount)
    let wasSpeaking = false

    speakingInterval = setInterval(() => {
      localAnalyser.getByteFrequencyData(dataArray)
      const avg = dataArray.reduce((a, b) => a + b, 0) / dataArray.length
      const isSpeaking = avg > 15 // threshold

      if (isSpeaking !== wasSpeaking) {
        wasSpeaking = isSpeaking
        if (onStoreUpdate) onStoreUpdate({ isSpeaking })
      }
    }, 100)
  } catch {
    // AudioContext not available
  }
}

const remoteAnalysers = new Map()

function startRemoteSpeakingDetection(userID, stream) {
  if (remoteAnalysers.has(userID)) return

  try {
    const ctx = new AudioContext()
    const analyser = ctx.createAnalyser()
    analyser.fftSize = 512
    const source = ctx.createMediaStreamSource(stream)
    source.connect(analyser)

    const dataArray = new Uint8Array(analyser.frequencyBinCount)
    let wasSpeaking = false

    const interval = setInterval(() => {
      analyser.getByteFrequencyData(dataArray)
      const avg = dataArray.reduce((a, b) => a + b, 0) / dataArray.length
      const isSpeaking = avg > 15

      if (isSpeaking !== wasSpeaking) {
        wasSpeaking = isSpeaking
        const peer = peers.get(userID)
        if (peer) {
          peer.isSpeaking = isSpeaking
          updatePeerList()
        }
      }
    }, 100)

    remoteAnalysers.set(userID, { ctx, interval })
  } catch {
    // AudioContext not available
  }
}

export function cleanupP2P() {
  leaveRoomP2P()
  for (const [_id, { ctx, interval }] of remoteAnalysers) {
    clearInterval(interval)
    ctx.close().catch(() => {})
  }
  remoteAnalysers.clear()
}
