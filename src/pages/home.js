/*
 * Home Page
 *
 * Serves as the forum landing page, showing a welcome banner and the most recent discussions to engage visitors.
 *
 * It must:
 * - Display a branded welcome section with the forum name and description
 * - List the most recent threads across all categories with author, category, reply count, and last activity
 * - Show pinned threads prominently with visual badges
 * - Indicate federated Forumline users with a globe icon on their avatar
 */

import { api } from '../lib/api.js'
import { getConfig } from '../lib/config.js'
import { avatarHTML } from '../components/avatar.js'
import { formatRelativeTime } from '../lib/date.js'

export function renderHome(container) {
  // eslint-disable-next-line no-unsanitized/property -- static template with skeleton loader
  container.innerHTML = `
    <div class="mb-8 bg-gradient-to-r from-indigo-900/50 to-purple-900/50 rounded-2xl p-8 border border-indigo-800/30">
      <h1 class="text-2xl font-bold mb-2">Welcome to ${getConfig().name}</h1>
      <p class="text-slate-300">A modern forum with real-time chat and voice rooms.</p>
    </div>
    <h2 class="text-lg font-semibold mb-4">Recent Discussions</h2>
    <div id="thread-list">
      <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl overflow-hidden divide-y divide-slate-700/50">
        ${[1,2,3,4,5].map(() => `
          <div class="flex items-start gap-3 px-3 py-3 sm:gap-4 sm:px-4 sm:py-4 animate-pulse">
            <div class="h-10 w-10 rounded-full bg-slate-700 shrink-0"></div>
            <div class="flex-1 space-y-2">
              <div class="h-3 w-16 bg-slate-700 rounded"></div>
              <div class="h-4 w-3/4 bg-slate-700 rounded"></div>
              <div class="flex gap-2"><div class="h-3 w-20 bg-slate-700 rounded"></div><div class="h-3 w-12 bg-slate-700 rounded"></div></div>
            </div>
          </div>
        `).join('')}
      </div>
    </div>
  `

  api.getThreads(20).then(threads => {
    const list = container.querySelector('#thread-list')
    if (!threads?.length) {
      list.innerHTML = '<p class="text-slate-400 text-center py-8">No discussions yet.</p>'
      return
    }

    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
    list.innerHTML = `<div class="bg-slate-800/50 border border-slate-700/50 rounded-xl overflow-hidden divide-y divide-slate-700/50">
      ${threads.map(t => `
        <a href="/t/${t.id}" class="flex items-start gap-3 px-3 py-3 sm:gap-4 sm:px-4 sm:py-4 transition-colors hover:bg-slate-700/30">
          ${avatarHTML({ avatarUrl: t.image_url || t.author?.avatar_url, size: 40 })}
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-1.5 flex-wrap sm:gap-2">
              ${t.is_pinned ? '<span class="text-[10px] sm:text-xs bg-amber-500/20 text-amber-400 px-1.5 py-0.5 rounded font-medium">Pinned</span>' : ''}
              <span class="text-[10px] sm:text-xs bg-slate-700 text-slate-400 px-1.5 py-0.5 rounded">${escapeHTML(t.category?.name || '')}</span>
            </div>
            <h3 class="mt-1 text-sm sm:text-base font-medium text-white line-clamp-2 sm:line-clamp-1">${escapeHTML(t.title)}</h3>
            <div class="mt-1 flex items-center gap-1.5 sm:gap-3 text-xs sm:text-sm text-slate-400 flex-wrap">
              <span class="flex items-center gap-1">
                ${avatarHTML({ avatarUrl: t.author?.avatar_url, size: 16, showGlobe: !!t.author?.forumline_id })}
                ${escapeHTML(t.author?.display_name || t.author?.username || 'Unknown')}
              </span>
              <span class="hidden sm:inline">&middot;</span>
              <span>${formatRelativeTime(t.created_at)}</span>
              <span>&middot;</span>
              <span>${t.post_count || 0} ${t.post_count === 1 ? 'reply' : 'replies'}</span>
            </div>
          </div>
          <div class="hidden sm:block shrink-0 text-right text-sm">
            <div class="text-slate-400">Last activity</div>
            <div class="text-slate-300">${formatRelativeTime(t.last_post_at || t.updated_at)}</div>
          </div>
        </a>
      `).join('')}
    </div>`
  }).catch(() => {
    container.querySelector('#thread-list').innerHTML = `
      <div class="text-center py-8">
        <p class="text-red-400">Failed to load threads.</p>
        <button id="retry-home" class="mt-2 text-sm text-indigo-400 hover:text-indigo-300">Try again</button>
      </div>
    `
    container.querySelector('#retry-home')?.addEventListener('click', () => renderHome(container))
  })
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
