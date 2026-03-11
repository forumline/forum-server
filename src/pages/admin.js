/*
 * Admin Dashboard
 *
 * Gives forum administrators an overview of community health and tools to manage users and content.
 *
 * It must:
 * - Restrict access to users with admin privileges only
 * - Display key community metrics (total users, threads, posts) on an overview tab
 * - Provide a searchable user list with role badges and join dates
 * - Reserve space for future content moderation and report review tools
 */

import { api } from '../lib/api.js'
import { authStore } from '../lib/auth.js'
import { avatarHTML } from '../components/avatar.js'
import { formatRelativeTime } from '../lib/date.js'

export function renderAdmin(container) {
  const { user } = authStore.get()
  if (!user?.is_admin) {
    container.innerHTML = `
      <div class="text-center py-16">
        <h1 class="text-2xl font-bold mb-2">Access Denied</h1>
        <p class="text-slate-400">You don't have permission to view this page.</p>
      </div>
    `
    return
  }

  let tab = 'overview'

  async function render() {
    // eslint-disable-next-line no-unsanitized/property -- static template
    container.innerHTML = `
      <h1 class="text-2xl font-bold mb-6">Admin Dashboard</h1>
      <div class="flex gap-2 mb-6">
        ${['overview', 'users', 'content', 'reports'].map(t => `
          <button class="tab-btn px-4 py-2 rounded-lg text-sm font-medium transition-colors ${t === tab ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}" data-tab="${t}">${t.charAt(0).toUpperCase() + t.slice(1)}</button>
        `).join('')}
      </div>
      <div id="admin-content"></div>
    `

    container.querySelectorAll('.tab-btn').forEach(btn => {
      btn.addEventListener('click', () => { tab = btn.dataset.tab; render() })
    })

    const content = container.querySelector('#admin-content')

    if (tab === 'overview') {
      try {
        const stats = await api.getAdminStats()
        // eslint-disable-next-line no-unsanitized/property -- static template, values from trusted API
        content.innerHTML = `
          <div class="grid gap-4 sm:grid-cols-3">
            ${statCard('Users', stats.totalUsers, 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z')}
            ${statCard('Threads', stats.totalThreads, 'M19 20H5a2 2 0 01-2-2V6a2 2 0 012-2h10a2 2 0 012 2v1m2 13a2 2 0 01-2-2V7m2 13a2 2 0 002-2V9a2 2 0 00-2-2h-2m-4-3H9M7 16h6M7 8h6v4H7V8z')}
            ${statCard('Posts', stats.totalPosts, 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z')}
          </div>
        `
      } catch {
        content.innerHTML = '<p class="text-red-400">Failed to load stats.</p>'
      }
    } else if (tab === 'users') {
      try {
        const users = await api.getAdminUsers()
        let searchQuery = ''

        function renderUsers() {
          const filtered = searchQuery
            ? users.filter(u => u.username.toLowerCase().includes(searchQuery) || (u.display_name || '').toLowerCase().includes(searchQuery))
            : users

          // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
          content.innerHTML = `
            <div class="mb-4">
              <input id="user-search" type="text" placeholder="Search users..." value="${searchQuery}" class="w-full max-w-sm px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
            </div>
            <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl overflow-hidden">
              <table class="w-full text-sm">
                <thead><tr class="border-b border-slate-700"><th class="px-4 py-3 text-left text-slate-400 font-medium">User</th><th class="px-4 py-3 text-left text-slate-400 font-medium hidden sm:table-cell">Role</th><th class="px-4 py-3 text-left text-slate-400 font-medium hidden sm:table-cell">Joined</th></tr></thead>
                <tbody>
                  ${filtered.map(u => `
                    <tr class="border-b border-slate-700/50 hover:bg-slate-800/50">
                      <td class="px-4 py-3">
                        <div class="flex items-center gap-2">
                          ${avatarHTML({ avatarUrl: u.avatar_url, size: 28 })}
                          <div>
                            <a href="/u/${u.username}" class="font-medium hover:text-indigo-400">${escapeHTML(u.display_name || u.username)}</a>
                            <div class="text-xs text-slate-500">@${escapeHTML(u.username)}</div>
                          </div>
                        </div>
                      </td>
                      <td class="px-4 py-3 hidden sm:table-cell">
                        <span class="text-xs px-2 py-0.5 rounded ${u.is_admin ? 'bg-amber-600/20 text-amber-400' : 'bg-slate-700 text-slate-400'}">${u.is_admin ? 'Admin' : 'User'}</span>
                      </td>
                      <td class="px-4 py-3 text-slate-500 hidden sm:table-cell">${formatRelativeTime(u.created_at)}</td>
                    </tr>
                  `).join('')}
                </tbody>
              </table>
            </div>
          `

          content.querySelector('#user-search')?.addEventListener('input', (e) => {
            searchQuery = e.target.value.toLowerCase()
            renderUsers()
          })
        }

        renderUsers()
      } catch {
        content.innerHTML = '<p class="text-red-400">Failed to load users.</p>'
      }
    } else if (tab === 'content') {
      content.innerHTML = '<p class="text-slate-400 text-center py-8">Content management coming soon.</p>'
    } else if (tab === 'reports') {
      content.innerHTML = '<p class="text-slate-400 text-center py-8">No reports to review.</p>'
    }
  }

  render()
}

function statCard(label, value, iconPath) {
  return `
    <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-6">
      <div class="flex items-center gap-3">
        <div class="p-2 bg-indigo-600/20 rounded-lg">
          <svg class="w-6 h-6 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="${iconPath}"/></svg>
        </div>
        <div>
          <div class="text-2xl font-bold">${(value || 0).toLocaleString()}</div>
          <div class="text-sm text-slate-400">${label}</div>
        </div>
      </div>
    </div>
  `
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
