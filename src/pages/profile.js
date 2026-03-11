/*
 * User Profile Page
 *
 * Displays a member's public profile so other users can see their activity and identity within the community.
 *
 * It must:
 * - Show the user's avatar, display name, username, bio, and join date
 * - Display tabs for the user's threads and posts with counts
 * - Indicate federated Forumline users with a globe badge on their avatar
 * - Offer a "Message" button to start a direct message conversation (visible to other logged-in users)
 */

import { api } from '../lib/api.js'
import { authStore } from '../lib/auth.js'
import { avatarHTML } from '../components/avatar.js'
import { formatRelativeTime, formatDate } from '../lib/date.js'

export function renderProfile(container, { username }) {
  container.innerHTML = '<div class="animate-pulse"><div class="h-32 bg-slate-800 rounded-xl"></div></div>'

  api.getProfileByUsername(username).then(async profile => {
    if (!profile) {
      container.innerHTML = '<p class="text-center py-8 text-slate-400">User not found.</p>'
      return
    }

    const { user } = authStore.get()
    const [threads, posts] = await Promise.all([
      api.getUserThreads(profile.id),
      api.getUserPosts(profile.id),
    ])

    let tab = 'threads'

    function render() {
      const showMessage = user && profile.id !== user.id

      // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
      container.innerHTML = `
        <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-6 mb-6">
          <div class="flex items-start gap-4">
            ${avatarHTML({ avatarUrl: profile.avatar_url, size: 64, showGlobe: !!profile.forumline_id })}
            <div class="flex-1">
              <h1 class="text-xl font-bold">${escapeHTML(profile.display_name || profile.username)}</h1>
              <p class="text-sm text-slate-400">@${escapeHTML(profile.username)}</p>
              ${profile.bio ? `<p class="text-sm text-slate-300 mt-2">${escapeHTML(profile.bio)}</p>` : ''}
              <div class="flex items-center gap-4 mt-3 text-xs text-slate-500">
                <span>${threads.length} threads</span>
                <span>${posts.length} posts</span>
                <span>Joined ${formatDate(profile.created_at)}</span>
              </div>
              ${showMessage ? `
                <a href="/dm/${profile.id}" class="mt-3 inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors">
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
                  Message
                </a>
              ` : ''}
            </div>
          </div>
        </div>

        <div class="flex gap-2 mb-4">
          <button class="tab-btn px-4 py-2 rounded-lg text-sm font-medium transition-colors ${tab === 'threads' ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}" data-tab="threads">Threads (${threads.length})</button>
          <button class="tab-btn px-4 py-2 rounded-lg text-sm font-medium transition-colors ${tab === 'posts' ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}" data-tab="posts">Posts (${posts.length})</button>
        </div>

        <div id="tab-content" class="space-y-2">
          ${tab === 'threads' ? renderThreads(threads) : renderPosts(posts)}
        </div>
      `

      container.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          tab = btn.dataset.tab
          render()
        })
      })
    }

    render()
  }).catch(() => {
    container.innerHTML = '<p class="text-red-400 text-center py-8">Failed to load profile.</p>'
  })
}

function renderThreads(threads) {
  if (!threads.length) return '<p class="text-slate-400 text-center py-8">No threads yet.</p>'
  return threads.map(t => `
    <a href="/t/${t.id}" class="block bg-slate-800/50 border border-slate-700/50 rounded-xl p-4 hover:bg-slate-800 transition-colors">
      <div class="flex items-center gap-2 mb-1">
        ${t.is_pinned ? '<span class="text-xs bg-amber-600/20 text-amber-400 px-1.5 py-0.5 rounded font-medium">Pinned</span>' : ''}
        <h3 class="font-semibold">${escapeHTML(t.title)}</h3>
      </div>
      <div class="text-xs text-slate-400">
        in <span class="text-indigo-400">${escapeHTML(t.category?.name || '')}</span> &middot; ${formatRelativeTime(t.created_at)} &middot; ${t.post_count || 0} replies
      </div>
    </a>
  `).join('')
}

function renderPosts(posts) {
  if (!posts.length) return '<p class="text-slate-400 text-center py-8">No posts yet.</p>'
  return posts.map(p => `
    <a href="/t/${p.thread_id}#post-${p.id}" class="block bg-slate-800/50 border border-slate-700/50 rounded-xl p-4 hover:bg-slate-800 transition-colors">
      <div class="text-sm text-slate-200 line-clamp-2 mb-1">${escapeHTML(p.content)}</div>
      <div class="text-xs text-slate-500">${formatRelativeTime(p.created_at)}</div>
    </a>
  `).join('')
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
