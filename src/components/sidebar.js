/*
 * Sidebar Navigation
 *
 * Provides persistent left-hand navigation so users can quickly move between forum categories, chat channels, and voice rooms.
 *
 * It must:
 * - List all categories, chat channels, and voice rooms with active-page highlighting
 * - Show live participant counts next to voice rooms so users can see where conversations are happening
 * - Display a connected voice room indicator with a leave button when the user is in a call
 * - Render both a desktop (always-visible) and mobile (overlay) version of the sidebar
 * - Auto-refresh when voice state or the current route changes
 */

import { api } from '../lib/api.js'
import { voiceStore, leaveRoom } from '../lib/voice.js'
import { authStore } from '../lib/auth.js'
import { getCurrentPath, onRouteChange } from '../router.js'
import { avatarHTML } from './avatar.js'

let categories = []
let channels = []
let rooms = []

export async function loadSidebarData() {
  try {
    [categories, channels, rooms] = await Promise.all([
      api.getCategories(),
      api.getChannels(),
      api.getVoiceRooms(),
    ])
  } catch {}
}

export function renderSidebarContent(container) {
  const path = getCurrentPath()
  const { user } = authStore.get()
  const voice = voiceStore.get()

  // eslint-disable-next-line no-unsanitized/property -- static template, user content escaped via escapeHTML()
  container.innerHTML = `
    <nav class="flex flex-col gap-1 p-3">
      <a href="/" class="flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${path === '/' ? 'bg-slate-700/50 text-white' : 'text-slate-300 hover:bg-slate-800 hover:text-white'}">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>
        Home
      </a>

      ${categories.length ? `
        <div class="mt-4 mb-1 px-3 text-xs font-semibold text-slate-500 uppercase tracking-wider">Categories</div>
        ${categories.map(c => `
          <a href="/c/${c.slug}" class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm transition-colors ${path === `/c/${c.slug}` ? 'bg-slate-700/50 text-white' : 'text-slate-400 hover:bg-slate-800 hover:text-white'}">
            <span class="w-2 h-2 rounded-full bg-indigo-500"></span>
            ${escapeHTML(c.name)}
          </a>
        `).join('')}
      ` : ''}

      ${channels.length ? `
        <div class="mt-4 mb-1 px-3 text-xs font-semibold text-slate-500 uppercase tracking-wider">Chat</div>
        ${channels.map(ch => `
          <a href="/chat/${ch.id}" class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm transition-colors ${path === `/chat/${ch.id}` ? 'bg-slate-700/50 text-white' : 'text-slate-400 hover:bg-slate-800 hover:text-white'}">
            <span class="text-slate-500">#</span>
            ${escapeHTML(ch.name)}
          </a>
        `).join('')}
      ` : ''}

      ${rooms.length ? `
        <div class="mt-4 mb-1 px-3 text-xs font-semibold text-slate-500 uppercase tracking-wider">Voice</div>
        ${rooms.map(r => {
          const info = voice.roomParticipantCounts[r.slug]
          const isActive = voice.connectedRoomSlug === r.slug
          return `
            <a href="/voice/${r.id}" class="flex items-center justify-between px-3 py-1.5 rounded-lg text-sm transition-colors ${isActive ? 'bg-green-900/30 text-green-400' : path === `/voice/${r.id}` ? 'bg-slate-700/50 text-white' : 'text-slate-400 hover:bg-slate-800 hover:text-white'}">
              <div class="flex items-center gap-2">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"/></svg>
                ${escapeHTML(r.name)}
              </div>
              ${info && info.count > 0 ? `<span class="text-xs text-green-400">${info.count}</span>` : ''}
            </a>
          `
        }).join('')}
      ` : ''}

      ${voice.isConnected ? `
        <div class="mt-2 mx-3 p-2 bg-green-900/20 border border-green-800/30 rounded-lg">
          <div class="flex items-center justify-between">
            <div class="text-xs text-green-400 font-medium">${escapeHTML(voice.connectedRoomName || '')}</div>
            <button id="sidebar-leave-voice" class="text-xs text-red-400 hover:text-red-300">Leave</button>
          </div>
        </div>
      ` : ''}
    </nav>
  `

  const leaveBtn = container.querySelector('#sidebar-leave-voice')
  if (leaveBtn) leaveBtn.addEventListener('click', leaveRoom)
}

export function renderSidebar(container) {
  container.innerHTML = `
    <aside class="hidden lg:block w-60 flex-shrink-0 border-r border-slate-700/50 h-[calc(100vh-3.5rem)] sticky top-14 overflow-y-auto">
      <div id="sidebar-content"></div>
    </aside>
  `
  renderSidebarContent(container.querySelector('#sidebar-content'))
}

export function renderMobileSidebar(container) {
  container.innerHTML = `
    <div id="mobile-sidebar" class="hidden fixed inset-0 z-50 lg:hidden">
      <div id="mobile-sidebar-overlay" class="absolute inset-0 bg-black/60"></div>
      <div class="absolute left-0 top-0 bottom-0 w-72 bg-slate-900 border-r border-slate-700/50 overflow-y-auto">
        <div id="mobile-sidebar-content" class="pt-14"></div>
      </div>
    </div>
  `
  renderSidebarContent(container.querySelector('#mobile-sidebar-content'))

  const overlay = container.querySelector('#mobile-sidebar-overlay')
  if (overlay) {
    overlay.addEventListener('click', () => {
      container.querySelector('#mobile-sidebar')?.classList.add('hidden')
    })
  }
}

export function closeMobileSidebar() {
  document.getElementById('mobile-sidebar')?.classList.add('hidden')
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str
  return div.innerHTML
}

// Re-render sidebar when voice state or route changes
function refreshSidebar() {
  const el = document.getElementById('sidebar-content')
  if (el) renderSidebarContent(el)
  const mel = document.getElementById('mobile-sidebar-content')
  if (mel) renderSidebarContent(mel)
}

voiceStore.subscribe(refreshSidebar)
onRouteChange(refreshSidebar)
