/*
 * Category Page
 *
 * Displays all threads within a specific forum category so users can browse discussions by topic.
 *
 * It must:
 * - Show the category name, description, and a list of its threads with author and reply counts
 * - Let authenticated users follow or unfollow the category to control their notification preferences
 * - Provide a "New Thread" button for authenticated users to start a discussion in this category
 * - Highlight pinned and locked threads with visual badges
 */

import { api } from '../lib/api.js'
import { authStore } from '../lib/auth.js'
import { avatarHTML } from '../components/avatar.js'
import { formatRelativeTime } from '../lib/date.js'
import { toast } from '../lib/toast.js'

export function renderCategory(container, { categorySlug }) {
  container.innerHTML = '<div class="animate-pulse"><div class="h-8 w-48 bg-slate-800 rounded mb-4"></div><div class="h-20 bg-slate-800 rounded-xl"></div></div>'

  Promise.all([
    api.getCategory(categorySlug),
    api.getThreadsByCategory(categorySlug),
    api.getChannelFollows().catch(() => []),
  ]).then(([category, threads, follows]) => {
    if (!category) {
      container.innerHTML = '<p class="text-center py-8 text-slate-400">Category not found.</p>'
      return
    }

    const { user } = authStore.get()
    const isFollowing = follows.some(f => f.category_id === category.id)

    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
    container.innerHTML = `
      <div class="mb-6">
        <div class="flex items-center justify-between flex-wrap gap-3">
          <div>
            <h1 class="text-2xl font-bold">${escapeHTML(category.name)}</h1>
            ${category.description ? `<p class="text-slate-400 mt-1">${escapeHTML(category.description)}</p>` : ''}
          </div>
          <div class="flex items-center gap-2">
            ${user ? `
              <button id="follow-btn" class="px-3 py-1.5 text-sm rounded-lg border transition-colors ${isFollowing ? 'bg-indigo-600 border-indigo-600 text-white' : 'border-slate-600 text-slate-300 hover:border-indigo-500'}">
                ${isFollowing ? 'Following' : 'Follow'}
              </button>
              <a href="/c/${categorySlug}/new" class="px-3 py-1.5 text-sm bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition-colors">New Thread</a>
            ` : ''}
          </div>
        </div>
      </div>
      <div id="thread-list" class="space-y-2">
        ${threads.length ? threads.map(t => threadCard(t)).join('') : '<p class="text-slate-400 text-center py-8">No threads yet.</p>'}
      </div>
    `

    const followBtn = container.querySelector('#follow-btn')
    if (followBtn) {
      let following = isFollowing
      followBtn.addEventListener('click', async () => {
        try {
          if (following) {
            await api.unfollowCategory(category.id)
            following = false
            followBtn.textContent = 'Follow'
            followBtn.className = 'px-3 py-1.5 text-sm rounded-lg border transition-colors border-slate-600 text-slate-300 hover:border-indigo-500'
          } else {
            await api.followCategory(category.id)
            following = true
            followBtn.textContent = 'Following'
            followBtn.className = 'px-3 py-1.5 text-sm rounded-lg border transition-colors bg-indigo-600 border-indigo-600 text-white'
          }
        } catch {
          toast.error('Failed to update follow status')
        }
      })
    }
  }).catch(() => {
    container.innerHTML = `
      <div class="text-center py-8">
        <p class="text-red-400">Failed to load category.</p>
        <button id="retry-cat" class="mt-2 text-sm text-indigo-400 hover:text-indigo-300">Try again</button>
      </div>
    `
    container.querySelector('#retry-cat')?.addEventListener('click', () => renderCategory(container, { categorySlug }))
  })
}

function threadCard(t) {
  return `
    <a href="/t/${t.id}" class="block bg-slate-800/50 hover:bg-slate-800 border border-slate-700/50 rounded-xl p-4 transition-colors">
      <div class="flex items-start gap-3">
        ${avatarHTML({ avatarUrl: t.image_url || t.author?.avatar_url, size: 40 })}
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            ${t.is_pinned ? '<span class="text-xs bg-amber-600/20 text-amber-400 px-1.5 py-0.5 rounded font-medium">Pinned</span>' : ''}
            ${t.is_locked ? '<span class="text-xs bg-red-600/20 text-red-400 px-1.5 py-0.5 rounded font-medium">Locked</span>' : ''}
            <h3 class="font-semibold text-white truncate">${escapeHTML(t.title)}</h3>
          </div>
          <div class="flex items-center gap-3 mt-1 text-xs text-slate-400">
            <span>${escapeHTML(t.author?.display_name || t.author?.username || 'Unknown')}</span>
            <span>${formatRelativeTime(t.created_at)}</span>
            <span>${t.post_count || 0} replies</span>
          </div>
        </div>
      </div>
    </a>
  `
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
