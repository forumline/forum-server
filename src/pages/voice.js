/*
 * Voice Room Page
 *
 * Provides the UI for browsing, joining, and participating in live voice conversations.
 *
 * It must:
 * - List all available voice rooms with live participant counts and avatars
 * - Show a focused room view with participant grid, speaking indicators, and mute states
 * - Provide mute, deafen, screen share, and disconnect controls for connected users
 * - Display screen share video when a participant is sharing their screen
 * - Reactively update the UI as participants join, leave, or change state
 * - Require authentication to join a room but allow guests to view room status
 */

import { api } from '../lib/api.js'
import { authStore } from '../lib/auth.js'
import { voiceStore, joinRoom, leaveRoom, toggleMute, toggleDeafen, toggleScreenShare, getAvatarUrl } from '../lib/voice.js'
import { avatarHTML } from '../components/avatar.js'

export function renderVoice(container, { roomId } = {}) {
  const { user } = authStore.get()

  function render() {
    const voice = voiceStore.get()

    api.getVoiceRooms().then(rooms => {
      const selectedRoom = roomId ? rooms.find(r => r.id === roomId || r.slug === roomId) : null

      if (selectedRoom) {
        renderFocusedRoom(container, selectedRoom, voice, user)
      } else {
        renderRoomList(container, rooms, voice, user)
      }

      bindEventHandlers(container, voice)
    })
  }

  render()
  const unsub = voiceStore.subscribe(render)
  return () => unsub()
}

function renderRoomList(container, rooms, voice, user) {
  // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
  container.innerHTML = `
    <h1 class="text-2xl font-bold mb-6">Voice Rooms</h1>

    ${voice.isConnected ? renderActiveRoomBar(voice) : ''}
    ${voice.connectError ? `<div class="mb-4 p-3 bg-red-900/30 border border-red-800/30 rounded-lg text-red-400 text-sm">${escapeHTML(voice.connectError)}</div>` : ''}

    <div class="grid gap-3 sm:grid-cols-2">
      ${rooms.map(r => renderRoomCard(r, voice, user)).join('')}
    </div>
  `
}

function renderFocusedRoom(container, room, voice, user) {
  const isConnectedToThisRoom = voice.isConnected && voice.connectedRoomSlug === room.slug
  const info = voice.roomParticipantCounts[room.slug]

  // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
  container.innerHTML = `
    <div class="mb-4">
      <a href="/voice" class="text-sm text-slate-400 hover:text-white transition-colors">&larr; All Voice Rooms</a>
    </div>

    <div class="bg-slate-800/50 border ${isConnectedToThisRoom ? 'border-green-700/50' : 'border-slate-700/50'} rounded-xl p-6">
      <div class="flex items-center justify-between mb-6">
        <div class="flex items-center gap-3">
          <div class="rounded-lg ${isConnectedToThisRoom ? 'bg-green-500/20' : 'bg-slate-700/50'} p-2">
            <svg class="w-6 h-6 ${isConnectedToThisRoom ? 'text-green-400' : 'text-slate-400'}" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"/></svg>
          </div>
          <div>
            <h1 class="text-xl font-bold">${escapeHTML(room.name)}</h1>
            ${isConnectedToThisRoom
              ? `<span class="text-sm text-green-400">${voice.participants.length + 1} connected</span>`
              : info && info.count > 0
                ? `<span class="text-sm text-slate-400">${info.count} online</span>`
                : `<span class="text-sm text-slate-500">Empty</span>`
            }
          </div>
        </div>
      </div>

      ${isConnectedToThisRoom ? renderConnectedView(voice, user) : renderDisconnectedView(room, voice, user, info)}
    </div>

    ${voice.connectError ? `<div class="mt-4 p-3 bg-red-900/30 border border-red-800/30 rounded-lg text-red-400 text-sm">${escapeHTML(voice.connectError)}</div>` : ''}
  `
}

function renderConnectedView(voice, user) {
  return `
    ${voice.screenShareTrack ? `
      <div class="mb-4 bg-black rounded-lg overflow-hidden">
        <div class="text-xs text-slate-400 px-2 py-1">${escapeHTML(voice.screenShareParticipant?.name || '')} is sharing their screen</div>
        <video id="screen-share-video" autoplay playsinline class="w-full"></video>
      </div>
    ` : ''}

    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 mb-6">
      <div class="bg-slate-900/50 rounded-lg p-3 text-center relative">
        ${avatarHTML({ avatarUrl: user?.avatar, size: 40, className: 'mx-auto' })}
        <div class="text-xs mt-1 font-medium">${escapeHTML(user?.username || 'You')}</div>
        ${voice.isSpeaking ? '<div class="absolute inset-0 rounded-lg border-2 border-green-400 animate-pulse pointer-events-none"></div>' : ''}
        ${voice.isMuted ? '<div class="absolute top-1 right-1 w-4 h-4 bg-red-600 rounded-full flex items-center justify-center"><svg class="w-2.5 h-2.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M6 18L18 6"/></svg></div>' : ''}
      </div>
      ${voice.participants.map(p => `
        <div class="bg-slate-900/50 rounded-lg p-3 text-center relative">
          ${avatarHTML({ avatarUrl: p.avatarUrl, size: 40, className: 'mx-auto' })}
          <div class="text-xs mt-1 font-medium">${escapeHTML(p.name)}</div>
          ${p.isSpeaking ? '<div class="absolute inset-0 rounded-lg border-2 border-green-400 animate-pulse pointer-events-none"></div>' : ''}
          ${p.isMuted ? '<div class="absolute top-1 right-1 w-4 h-4 bg-red-600 rounded-full flex items-center justify-center"><svg class="w-2.5 h-2.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M6 18L18 6"/></svg></div>' : ''}
        </div>
      `).join('')}
    </div>

    <div class="flex items-center justify-center gap-2">
      <button id="voice-mute" class="px-4 py-2 rounded-lg text-sm font-medium transition-colors ${voice.isMuted ? 'bg-red-600 hover:bg-red-500 text-white' : 'bg-slate-700 hover:bg-slate-600 text-white'}">
        ${voice.isMuted ? 'Unmute' : 'Mute'}
      </button>
      <button id="voice-deafen" class="px-4 py-2 rounded-lg text-sm font-medium transition-colors ${voice.isDeafened ? 'bg-red-600 hover:bg-red-500 text-white' : 'bg-slate-700 hover:bg-slate-600 text-white'}">
        ${voice.isDeafened ? 'Undeafen' : 'Deafen'}
      </button>
      <button id="voice-share" class="px-4 py-2 rounded-lg text-sm font-medium transition-colors ${voice.isScreenSharing ? 'bg-green-600 hover:bg-green-500 text-white' : 'bg-slate-700 hover:bg-slate-600 text-white'}">
        ${voice.isScreenSharing ? 'Stop Share' : 'Share Screen'}
      </button>
      <button id="voice-disconnect" class="px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded-lg text-sm font-medium transition-colors">
        Disconnect
      </button>
    </div>
  `
}

function renderDisconnectedView(room, voice, user, info) {
  return `
    ${info && info.count > 0 ? `
      <div class="mb-6">
        <div class="text-sm text-slate-400 mb-2">Currently in this room:</div>
        <div class="flex flex-wrap gap-2">
          ${info.identities.map((id, i) => `
            <div class="flex items-center gap-2 bg-slate-900/50 rounded-lg px-3 py-2">
              ${avatarHTML({ avatarUrl: getAvatarUrl(id), size: 24 })}
              <span class="text-sm">${escapeHTML(info.names?.[i] || 'User')}</span>
            </div>
          `).join('')}
        </div>
      </div>
    ` : `
      <div class="mb-6 text-center py-4">
        <p class="text-slate-500">No one is in this room yet. Be the first to join!</p>
      </div>
    `}

    ${user ? `
      <button class="join-btn w-full py-3 rounded-lg text-sm font-medium transition-colors bg-indigo-600 hover:bg-indigo-500 text-white" data-slug="${room.slug}" data-name="${escapeHTML(room.name)}" data-is-active="false">
        ${voice.isConnecting ? 'Connecting...' : 'Join Voice'}
      </button>
    ` : '<p class="text-center text-slate-500"><a href="/login" class="text-indigo-400">Sign in</a> to join voice rooms</p>'}
  `
}

function renderActiveRoomBar(voice) {
  return `
    <div class="mb-4 bg-green-900/20 border border-green-800/30 rounded-xl p-3 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <div class="w-2 h-2 rounded-full bg-green-400 animate-pulse"></div>
        <span class="text-sm font-medium text-green-400">Connected to ${escapeHTML(voice.connectedRoomName || '')}</span>
        <span class="text-xs text-slate-500">${voice.participants.length + 1} participants</span>
      </div>
      <button id="voice-disconnect" class="text-xs text-red-400 hover:text-red-300 font-medium">Disconnect</button>
    </div>
  `
}

function renderRoomCard(r, voice, _user) {
  const info = voice.roomParticipantCounts[r.slug]
  const isActive = voice.connectedRoomSlug === r.slug
  return `
    <a href="/voice/${r.id}" class="block bg-slate-800/50 border ${isActive ? 'border-green-700/50' : 'border-slate-700/50'} rounded-xl p-4 hover:bg-slate-800 transition-colors">
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-2">
          <svg class="w-5 h-5 ${isActive ? 'text-green-400' : 'text-slate-500'}" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"/></svg>
          <h3 class="font-semibold">${escapeHTML(r.name)}</h3>
        </div>
        ${info && info.count > 0 ? `<span class="text-sm text-green-400">${info.count} online</span>` : ''}
      </div>
      ${info && info.count > 0 ? `
        <div class="flex flex-wrap gap-1">
          ${info.identities.slice(0, 5).map(id => avatarHTML({ avatarUrl: getAvatarUrl(id), size: 24 })).join('')}
          ${info.count > 5 ? `<span class="text-xs text-slate-500 self-center">+${info.count - 5}</span>` : ''}
        </div>
      ` : `<p class="text-xs text-slate-500">Empty</p>`}
    </a>
  `
}

function bindEventHandlers(container, voice) {
  container.querySelectorAll('.join-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.preventDefault()
      if (btn.dataset.isActive === 'true') {
        leaveRoom()
      } else {
        joinRoom(btn.dataset.slug, btn.dataset.name)
      }
    })
  })

  const muteBtn = container.querySelector('#voice-mute')
  if (muteBtn) muteBtn.addEventListener('click', toggleMute)

  const deafenBtn = container.querySelector('#voice-deafen')
  if (deafenBtn) deafenBtn.addEventListener('click', toggleDeafen)

  const shareBtn = container.querySelector('#voice-share')
  if (shareBtn) shareBtn.addEventListener('click', toggleScreenShare)

  const disconnectBtn = container.querySelector('#voice-disconnect')
  if (disconnectBtn) disconnectBtn.addEventListener('click', leaveRoom)

  // Screen share video
  if (voice.screenShareTrack) {
    const video = container.querySelector('#screen-share-video')
    if (video) {
      const stream = new MediaStream([voice.screenShareTrack])
      video.srcObject = stream
    }
  }
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
