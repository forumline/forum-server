/*
 * Bookmarks Page
 *
 * Shows a user's saved threads so they can quickly return to discussions they care about.
 *
 * It must:
 * - Require authentication and prompt sign-in for logged-out visitors
 * - Display all bookmarked threads with their category, author, and save date
 * - Allow users to remove individual bookmarks inline
 * - Handle empty state gracefully with a link back to browsing threads
 */

import { api } from '../lib/api.js'
import { authStore } from '../lib/auth.js'
import { avatarHTML } from '../components/avatar.js'
import { formatRelativeTime } from '../lib/date.js'
import { toast } from '../lib/toast.js'

export function renderBookmarks(container) {
  const { user } = authStore.get()
  if (!user) {
    container.innerHTML = '<p class="text-center py-8 text-slate-400"><a href="/login" class="text-indigo-400">Sign in</a> to view bookmarks.</p>'
    return
  }

  container.innerHTML = '<div class="animate-pulse"><div class="h-8 w-48 bg-slate-800 rounded mb-4"></div><div class="h-20 bg-slate-800 rounded-xl"></div></div>'

  let bookmarks = []

  function render() {
    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
    container.innerHTML = `
      <h1 class="text-2xl font-bold mb-2">Bookmarks</h1>
      <p class="text-sm text-slate-400 mb-6">${bookmarks.length} saved thread${bookmarks.length !== 1 ? 's' : ''}</p>

      <div class="space-y-2">
        ${bookmarks.length ? bookmarks.map(b => `
          <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-4 flex items-start gap-3">
            <a href="/t/${b.thread.id}" class="flex-1 min-w-0 flex items-start gap-3">
              ${avatarHTML({ avatarUrl: b.thread.image_url || b.thread.author?.avatar_url, size: 40 })}
              <div class="min-w-0">
                <h3 class="font-semibold text-white truncate">${escapeHTML(b.thread.title)}</h3>
                <div class="text-xs text-slate-400 mt-0.5">
                  <span class="text-indigo-400">${escapeHTML(b.thread.category?.name || '')}</span>
                  &middot; ${escapeHTML(b.thread.author?.display_name || '')}
                  &middot; saved ${formatRelativeTime(b.created_at)}
                </div>
              </div>
            </a>
            <button class="remove-bookmark p-1 text-slate-500 hover:text-red-400 transition-colors" data-id="${b.id}">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
            </button>
          </div>
        `).join('') : '<p class="text-slate-400 text-center py-8">No bookmarks yet. <a href="/" class="text-indigo-400 hover:text-indigo-300">Browse threads</a></p>'}
      </div>
    `

    container.querySelectorAll('.remove-bookmark').forEach(btn => {
      btn.addEventListener('click', async () => {
        const id = btn.dataset.id
        try {
          await api.removeBookmarkById(id)
          bookmarks = bookmarks.filter(b => b.id !== id)
          render()
        } catch {
          toast.error('Failed to remove bookmark')
        }
      })
    })
  }

  api.getBookmarksWithMeta().then(data => {
    bookmarks = data || []
    render()
  }).catch(() => {
    container.innerHTML = `
      <div class="text-center py-8">
        <p class="text-red-400">Failed to load bookmarks.</p>
        <button id="retry-bm" class="mt-2 text-sm text-indigo-400 hover:text-indigo-300">Try again</button>
      </div>
    `
    container.querySelector('#retry-bm')?.addEventListener('click', () => renderBookmarks(container))
  })
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
