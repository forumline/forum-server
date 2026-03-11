/*
 * Thread View Page
 *
 * Displays a full discussion thread with all its posts, and lets users participate by replying.
 *
 * It must:
 * - Show the thread title, category breadcrumb, optional image, and pinned/locked status
 * - Paginate posts and support replying to specific posts with quote context
 * - Allow authenticated users to bookmark/unbookmark the thread
 * - Receive new posts in real time via SSE and offer a "load new posts" prompt
 * - Prevent replies on locked threads and prompt sign-in for unauthenticated visitors
 * - Highlight the original poster with an "OP" badge on their posts
 */

import { api } from '../lib/api.js'
import { authStore } from '../lib/auth.js'
import { avatarHTML } from '../components/avatar.js'
import { formatRelativeTime } from '../lib/date.js'
import { toast } from '../lib/toast.js'
import { connectSSE } from '../lib/sse.js'

const POSTS_PER_PAGE = 5

export function renderThread(container, { threadId }) {
  container.innerHTML = '<div class="animate-pulse space-y-3"><div class="h-8 w-64 bg-slate-800 rounded"></div><div class="h-32 bg-slate-800 rounded-xl"></div></div>'

  let currentPage = 1
  let replyingTo = null
  let sseCleanup = null
  let pendingPosts = []

  Promise.all([
    api.getThread(threadId),
    api.getPosts(threadId),
    authStore.get().user ? api.isBookmarked(threadId).catch(() => false) : Promise.resolve(false),
  ]).then(([thread, posts, isBookmarked]) => {
    if (!thread) {
      container.innerHTML = '<p class="text-center py-8 text-slate-400">Thread not found.</p>'
      return
    }

    let bookmarked = isBookmarked
    const { user } = authStore.get()
    const totalPages = Math.ceil((posts?.length || 0) / POSTS_PER_PAGE)

    function render() {
      const start = (currentPage - 1) * POSTS_PER_PAGE
      const pagePosts = (posts || []).slice(start, start + POSTS_PER_PAGE)

      // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
      container.innerHTML = `
        <div class="mb-4">
          <div class="flex items-center gap-2 text-sm text-slate-400 mb-3">
            <a href="/" class="hover:text-indigo-400">Home</a>
            <span>/</span>
            <a href="/c/${thread.category?.slug || ''}" class="hover:text-indigo-400">${escapeHTML(thread.category?.name || '')}</a>
          </div>

          <div class="flex items-start justify-between gap-4">
            <div>
              <div class="flex items-center gap-2 flex-wrap mb-1">
                ${thread.is_pinned ? '<span class="text-xs bg-amber-600/20 text-amber-400 px-1.5 py-0.5 rounded font-medium">Pinned</span>' : ''}
                ${thread.is_locked ? '<span class="text-xs bg-red-600/20 text-red-400 px-1.5 py-0.5 rounded font-medium">Locked</span>' : ''}
              </div>
              <h1 class="text-2xl font-bold">${escapeHTML(thread.title)}</h1>
            </div>
            ${user ? `
              <button id="bookmark-btn" class="p-2 rounded-lg transition-colors ${bookmarked ? 'text-yellow-400 hover:text-yellow-300' : 'text-slate-400 hover:text-slate-200'}">
                <svg class="w-5 h-5" fill="${bookmarked ? 'currentColor' : 'none'}" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z"/></svg>
              </button>
            ` : ''}
          </div>

          ${thread.image_url ? `<img src="${thread.image_url}" alt="" class="mt-4 rounded-xl max-h-64 object-cover" />` : ''}

          <div class="mt-2 text-sm text-slate-400">
            ${posts?.length || 0} posts &middot; Page ${currentPage} of ${totalPages || 1}
          </div>
        </div>

        <div id="posts-container" class="space-y-4 mb-6">
          ${pagePosts.map(p => postHTML(p, thread, posts, user)).join('')}
        </div>

        ${totalPages > 1 ? `
          <div class="flex items-center justify-center gap-2 mb-6">
            ${currentPage > 1 ? `<button class="page-btn px-3 py-1 bg-slate-800 rounded text-sm hover:bg-slate-700" data-page="${currentPage - 1}">Prev</button>` : ''}
            ${Array.from({ length: totalPages }, (_, i) => i + 1).map(p =>
              `<button class="page-btn px-3 py-1 rounded text-sm ${p === currentPage ? 'bg-indigo-600 text-white' : 'bg-slate-800 hover:bg-slate-700'}" data-page="${p}">${p}</button>`
            ).join('')}
            ${currentPage < totalPages ? `<button class="page-btn px-3 py-1 bg-slate-800 rounded text-sm hover:bg-slate-700" data-page="${currentPage + 1}">Next</button>` : ''}
          </div>
        ` : ''}

        ${pendingPosts.length > 0 ? `
          <div class="mb-4 p-3 bg-indigo-900/30 border border-indigo-800/30 rounded-lg text-center">
            <button id="show-new-posts" class="text-sm text-indigo-400 hover:text-indigo-300">${pendingPosts.length} new post${pendingPosts.length > 1 ? 's' : ''} — click to load</button>
          </div>
        ` : ''}

        ${!thread.is_locked ? `
          <div id="reply-section" class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-4">
            ${user ? `
              ${replyingTo ? `
                <div class="mb-2 flex items-center gap-2 text-sm text-slate-400">
                  <span>Replying to ${escapeHTML(replyingTo.author?.display_name || replyingTo.author?.username || '')}</span>
                  <button id="cancel-reply" class="text-red-400 hover:text-red-300">Cancel</button>
                </div>
              ` : ''}
              <textarea id="reply-content" rows="4" placeholder="Write a reply..." class="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-white resize-none focus:outline-none focus:ring-2 focus:ring-indigo-500"></textarea>
              <div class="mt-2 flex justify-end">
                <button id="post-reply-btn" class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg text-sm font-medium transition-colors">Post Reply</button>
              </div>
            ` : `<p class="text-slate-400 text-center"><a href="/login" class="text-indigo-400 hover:text-indigo-300">Sign in</a> to reply.</p>`}
          </div>
        ` : '<p class="text-center text-slate-500 text-sm">This thread is locked.</p>'}
      `

      // Event handlers
      container.querySelectorAll('.page-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          currentPage = parseInt(btn.dataset.page)
          render()
        })
      })

      container.querySelectorAll('.reply-to-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          const postId = btn.dataset.postId
          replyingTo = posts.find(p => p.id === postId) || null
          render()
          container.querySelector('#reply-content')?.focus()
        })
      })

      const bookmarkBtn = container.querySelector('#bookmark-btn')
      if (bookmarkBtn) {
        bookmarkBtn.addEventListener('click', async () => {
          try {
            if (bookmarked) {
              await api.removeBookmark(threadId)
              bookmarked = false
            } else {
              await api.addBookmark(threadId)
              bookmarked = true
            }
            render()
          } catch { toast.error('Failed to update bookmark') }
        })
      }

      const cancelReply = container.querySelector('#cancel-reply')
      if (cancelReply) {
        cancelReply.addEventListener('click', () => { replyingTo = null; render() })
      }

      const postBtn = container.querySelector('#post-reply-btn')
      if (postBtn) {
        postBtn.addEventListener('click', async () => {
          const textarea = container.querySelector('#reply-content')
          const content = textarea?.value?.trim()
          if (!content) return

          postBtn.disabled = true
          postBtn.textContent = 'Posting...'

          try {
            await api.createPost({
              thread_id: threadId,
              author_id: user.id,
              content,
              reply_to_id: replyingTo?.id,
            })
            // Reload posts
            posts = await api.getPosts(threadId)
            currentPage = Math.ceil(posts.length / POSTS_PER_PAGE)
            replyingTo = null
            render()
          } catch (_err) {
            toast.error('Failed to post reply')
            postBtn.disabled = false
            postBtn.textContent = 'Post Reply'
          }
        })
      }

      const showNewBtn = container.querySelector('#show-new-posts')
      if (showNewBtn) {
        showNewBtn.addEventListener('click', async () => {
          posts = await api.getPosts(threadId)
          pendingPosts = []
          currentPage = Math.ceil(posts.length / POSTS_PER_PAGE)
          render()
        })
      }
    }

    render()

    // SSE for live updates
    sseCleanup = connectSSE(`/api/threads/${threadId}/stream`, (data) => {
      if (data?.type === 'new_post' && data.post) {
        if (!posts.find(p => p.id === data.post.id)) {
          pendingPosts.push(data.post)
          render()
        }
      }
    }, true)
  }).catch(() => {
    container.innerHTML = `
      <div class="text-center py-8">
        <p class="text-red-400">Failed to load thread.</p>
        <button id="retry-thread" class="mt-2 text-sm text-indigo-400 hover:text-indigo-300">Try again</button>
      </div>
    `
    container.querySelector('#retry-thread')?.addEventListener('click', () => renderThread(container, { threadId }))
  })

  return () => {
    if (sseCleanup) sseCleanup()
  }
}

function postHTML(post, thread, allPosts, user) {
  const isOP = post.author_id === thread.author_id
  const replyTo = post.reply_to_id ? allPosts.find(p => p.id === post.reply_to_id) : null

  return `
    <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-4" id="post-${post.id}">
      ${replyTo ? `
        <div class="mb-2 text-xs text-slate-500 flex items-center gap-1">
          <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"/></svg>
          Replying to ${escapeHTML(replyTo.author?.display_name || replyTo.author?.username || '')}
        </div>
      ` : ''}
      <div class="flex items-start gap-3">
        ${avatarHTML({ avatarUrl: post.author?.avatar_url, size: 36 })}
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-1">
            <a href="/u/${post.author?.username || ''}" class="font-semibold text-sm hover:text-indigo-400">${escapeHTML(post.author?.display_name || post.author?.username || 'Unknown')}</a>
            ${isOP ? '<span class="text-xs bg-indigo-600/30 text-indigo-400 px-1.5 py-0.5 rounded">OP</span>' : ''}
            <span class="text-xs text-slate-500">${formatRelativeTime(post.created_at)}</span>
          </div>
          <div class="text-sm text-slate-200 whitespace-pre-wrap">${escapeHTML(post.content)}</div>
          ${user && !thread.is_locked ? `
            <div class="mt-2">
              <button class="reply-to-btn text-xs text-slate-400 hover:text-indigo-400" data-post-id="${post.id}">Reply</button>
            </div>
          ` : ''}
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
