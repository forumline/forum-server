/*
 * Top Navigation Header
 *
 * Provides the persistent top bar with forum branding, search access, notifications, and user account controls.
 *
 * It must:
 * - Show the forum name and link to the home page for brand identity
 * - Display a live notification bell with unread count, updated in real time via SSE
 * - Provide a user menu with links to profile, bookmarks, settings, admin (if applicable), and sign out
 * - Show sign in / sign up links for unauthenticated visitors
 * - Toggle the mobile sidebar menu on small screens
 * - Support the "/" keyboard shortcut to jump to search
 */

import { authStore, signOut } from '../lib/auth.js'
import { navigate } from '../router.js'
import { avatarHTML } from './avatar.js'
import { getConfig } from '../lib/config.js'
import { api } from '../lib/api.js'
import { connectSSE } from '../lib/sse.js'
import { getAccessToken } from '../lib/auth.js'
import { formatRelativeTime } from '../lib/date.js'

let notificationCleanup = null
let notifications = []
let dropdownOpen = false
let userMenuOpen = false

export function renderHeader(container) {
  const { user } = authStore.get()
  const forumName = getConfig().name

  // eslint-disable-next-line no-unsanitized/property -- static template, user content escaped via escapeHTML()
  container.innerHTML = `
    <header class="sticky top-0 z-40 h-14 bg-slate-900/95 backdrop-blur border-b border-slate-700/50 px-4 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <button id="mobile-menu-btn" class="lg:hidden p-1.5 text-slate-400 hover:text-slate-200">
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/></svg>
        </button>
        <a href="/" class="flex items-center gap-2 text-lg font-bold text-white hover:text-indigo-400 transition-colors">
          <svg class="w-6 h-6 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
          ${forumName}
        </a>
      </div>

      <div class="flex items-center gap-2">
        <a href="/search" class="p-2 text-slate-400 hover:text-slate-200 transition-colors">
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
        </a>
        ${user ? `
          <div class="relative" id="notification-wrapper">
            <button id="notification-btn" class="p-2 text-slate-400 hover:text-slate-200 transition-colors relative">
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>
              <span id="notification-badge" class="hidden absolute -top-0.5 -right-0.5 w-4 h-4 bg-red-500 rounded-full text-xs flex items-center justify-center text-white font-bold"></span>
            </button>
            <div id="notification-dropdown" class="hidden absolute right-0 top-full mt-1 w-80 bg-slate-800 border border-slate-700 rounded-lg shadow-xl overflow-hidden">
              <div class="p-3 border-b border-slate-700 flex items-center justify-between">
                <h3 class="font-semibold text-sm">Notifications</h3>
                <button id="mark-all-read" class="text-xs text-indigo-400 hover:text-indigo-300">Mark all read</button>
              </div>
              <div id="notification-list" class="max-h-80 overflow-y-auto"></div>
            </div>
          </div>
          <div class="relative" id="user-menu-wrapper">
            <button id="user-menu-btn" class="flex items-center gap-2 p-1 rounded-lg hover:bg-slate-800 transition-colors">
              ${avatarHTML({ avatarUrl: user.avatar, size: 32 })}
            </button>
            <div id="user-menu-dropdown" class="hidden absolute right-0 top-full mt-1 w-48 bg-slate-800 border border-slate-700 rounded-lg shadow-xl overflow-hidden">
              <a href="/u/${user.username || ''}" class="block px-4 py-2 text-sm hover:bg-slate-700 transition-colors">Profile</a>
              <a href="/bookmarks" class="block px-4 py-2 text-sm hover:bg-slate-700 transition-colors">Bookmarks</a>
              <a href="/settings" class="block px-4 py-2 text-sm hover:bg-slate-700 transition-colors">Settings</a>
              ${user.is_admin ? '<a href="/admin" class="block px-4 py-2 text-sm hover:bg-slate-700 transition-colors">Admin</a>' : ''}
              <div class="border-t border-slate-700"></div>
              <button id="sign-out-btn" class="w-full text-left px-4 py-2 text-sm text-red-400 hover:bg-slate-700 transition-colors">Sign Out</button>
            </div>
          </div>
        ` : `
          <a href="/login" class="px-3 py-1.5 text-sm text-slate-300 hover:text-white transition-colors">Sign In</a>
          <a href="/register" class="px-3 py-1.5 text-sm bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition-colors">Sign Up</a>
        `}
      </div>
    </header>
  `

  // Mobile menu toggle
  const mobileBtn = container.querySelector('#mobile-menu-btn')
  if (mobileBtn) {
    mobileBtn.addEventListener('click', () => {
      document.getElementById('mobile-sidebar')?.classList.toggle('hidden')
    })
  }

  // User menu
  const userMenuBtn = container.querySelector('#user-menu-btn')
  const userMenuDropdown = container.querySelector('#user-menu-dropdown')
  if (userMenuBtn && userMenuDropdown) {
    userMenuBtn.addEventListener('click', (e) => {
      e.stopPropagation()
      userMenuOpen = !userMenuOpen
      userMenuDropdown.classList.toggle('hidden', !userMenuOpen)
    })
  }

  // Sign out
  const signOutBtn = container.querySelector('#sign-out-btn')
  if (signOutBtn) {
    signOutBtn.addEventListener('click', async () => {
      await signOut()
      navigate('/')
    })
  }

  // Notifications
  if (user) {
    setupNotifications(container)
  }

  // Close dropdowns on outside click
  document.addEventListener('click', (e) => {
    if (userMenuDropdown && !container.querySelector('#user-menu-wrapper')?.contains(e.target)) {
      userMenuDropdown.classList.add('hidden')
      userMenuOpen = false
    }
    const notifDropdown = container.querySelector('#notification-dropdown')
    if (notifDropdown && !container.querySelector('#notification-wrapper')?.contains(e.target)) {
      notifDropdown.classList.add('hidden')
      dropdownOpen = false
    }
  })

  // Close dropdowns on Escape + "/" to focus search
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (userMenuDropdown && userMenuOpen) {
        userMenuDropdown.classList.add('hidden')
        userMenuOpen = false
      }
      const notifDropdown = container.querySelector('#notification-dropdown')
      if (notifDropdown && dropdownOpen) {
        notifDropdown.classList.add('hidden')
        dropdownOpen = false
      }
    }
    if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(e.target.tagName)) {
      e.preventDefault()
      navigate('/search')
    }
  })
}

async function setupNotifications(container) {
  try {
    notifications = await api.getNotifications()
    updateNotificationUI(container)
  } catch {}

  // SSE for live notifications
  notificationCleanup = connectSSE('/api/forumline/notifications/stream', (data) => {
    if (data && data.id) {
      notifications = [data, ...notifications.filter(n => n.id !== data.id)]
      updateNotificationUI(container)
    }
  }, true)

  const notifBtn = container.querySelector('#notification-btn')
  const notifDropdown = container.querySelector('#notification-dropdown')
  if (notifBtn && notifDropdown) {
    notifBtn.addEventListener('click', (e) => {
      e.stopPropagation()
      dropdownOpen = !dropdownOpen
      notifDropdown.classList.toggle('hidden', !dropdownOpen)
    })
  }

  const markAllBtn = container.querySelector('#mark-all-read')
  if (markAllBtn) {
    markAllBtn.addEventListener('click', async () => {
      await api.markAllNotificationsRead()
      notifications = notifications.map(n => ({ ...n, read: true }))
      updateNotificationUI(container)
    })
  }
}

function updateNotificationUI(container) {
  const badge = container.querySelector('#notification-badge')
  const list = container.querySelector('#notification-list')
  if (!badge || !list) return

  const unread = notifications.filter(n => !n.read).length
  if (unread > 0) {
    badge.textContent = unread > 9 ? '9+' : unread
    badge.classList.remove('hidden')
  } else {
    badge.classList.add('hidden')
  }

  if (notifications.length === 0) {
    list.innerHTML = '<p class="p-4 text-sm text-slate-400 text-center">No notifications</p>'
    return
  }

  // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
  list.innerHTML = notifications.slice(0, 20).map(n => `
    <a href="${n.link || '#'}" class="block px-4 py-3 hover:bg-slate-700/50 transition-colors border-b border-slate-700/50 ${n.read ? 'opacity-60' : ''}" data-notif-id="${n.id}">
      <p class="text-sm font-medium">${escapeHTML(n.title)}</p>
      <p class="text-xs text-slate-400 mt-0.5">${escapeHTML(n.message)}</p>
      <p class="text-xs text-slate-500 mt-1">${formatRelativeTime(n.created_at)}</p>
    </a>
  `).join('')

  list.querySelectorAll('[data-notif-id]').forEach(el => {
    el.addEventListener('click', () => {
      const id = el.dataset.notifId
      if (!notifications.find(n => n.id === id)?.read) {
        api.markNotificationRead(id)
        notifications = notifications.map(n => n.id === id ? { ...n, read: true } : n)
        updateNotificationUI(container)
      }
    })
  })
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str
  return div.innerHTML
}

export function cleanupHeader() {
  if (notificationCleanup) { notificationCleanup(); notificationCleanup = null }
}
