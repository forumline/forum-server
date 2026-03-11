/*
 * Real-Time Chat Channel
 *
 * Provides a live messaging experience within forum chat channels, similar to Discord-style text chat.
 *
 * It must:
 * - Display messages grouped by author and date with avatars and timestamps
 * - Send messages instantly with optimistic UI updates (show before server confirms)
 * - Receive new messages from other users in real time via SSE without page refresh
 * - Auto-scroll to the latest messages while preserving scroll position when reading history
 * - Require authentication to send messages but allow guests to read
 */

import { api } from '../lib/api.js'
import { authStore } from '../lib/auth.js'
import { avatarHTML } from '../components/avatar.js'
import { formatTime, formatDateLabel } from '../lib/date.js'
import { connectSSE } from '../lib/sse.js'

export function renderChat(container, { channelId }) {
  const { user } = authStore.get()
  let sseCleanup = null
  let channels = []
  let messages = []
  let currentChannel = null

  // Add chat-active class to body
  document.body.classList.add('chat-active')

  async function loadChannels() {
    channels = await api.getChannels()
    if (!channelId && channels.length > 0) {
      currentChannel = channels[0]
    } else {
      currentChannel = channels.find(c => c.id === channelId) || null
    }
  }

  async function loadMessages() {
    if (!currentChannel) return
    messages = await api.getChatMessages(currentChannel.slug)
  }

  function render() {
    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
    container.innerHTML = `
      <div class="chat-page-wrapper flex flex-col">
        <div class="h-12 border-b border-slate-700/50 px-4 flex items-center gap-2 flex-shrink-0">
          <span class="text-slate-500">#</span>
          <span class="font-semibold">${escapeHTML(currentChannel?.name || '')}</span>
          ${currentChannel?.description ? `<span class="text-sm text-slate-500 hidden sm:inline">&mdash; ${escapeHTML(currentChannel.description)}</span>` : ''}
        </div>

        <div id="messages-area" class="flex-1 overflow-y-auto px-4 py-2">
          ${renderMessages()}
        </div>

        ${user ? `
          <div class="px-4 py-3 border-t border-slate-700/50 flex-shrink-0">
            <div class="flex gap-2">
              <input id="chat-input" type="text" placeholder="Message #${escapeHTML(currentChannel?.name || '')}" class="flex-1 px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
              <button id="chat-send" class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition-colors">Send</button>
            </div>
          </div>
        ` : `
          <div class="px-4 py-3 border-t border-slate-700/50 text-center text-slate-400 text-sm">
            <a href="/login" class="text-indigo-400 hover:text-indigo-300">Sign in</a> to chat.
          </div>
        `}
      </div>
    `

    // Scroll to bottom
    const area = container.querySelector('#messages-area')
    if (area) area.scrollTop = area.scrollHeight

    // Send message
    const input = container.querySelector('#chat-input')
    const sendBtn = container.querySelector('#chat-send')

    function sendMessage() {
      const content = input?.value?.trim()
      if (!content || !currentChannel || !user) return
      input.value = ''

      // Optimistic add
      const optimistic = {
        id: 'temp-' + Date.now(),
        channel_id: currentChannel.id,
        author_id: user.id,
        content,
        created_at: new Date().toISOString(),
        author: { id: user.id, username: user.username, display_name: user.username, avatar_url: user.avatar },
      }
      messages.push(optimistic)
      renderMessagesInPlace()

      api.sendChatMessage({ channel_id: currentChannel.id, author_id: user.id, content }).catch(() => {
        messages = messages.filter(m => m.id !== optimistic.id)
        renderMessagesInPlace()
      })
    }

    if (sendBtn) sendBtn.addEventListener('click', sendMessage)
    if (input) {
      input.addEventListener('keydown', (e) => { if (e.key === 'Enter') sendMessage() })
      input.focus()
    }

    // SSE for live messages
    if (currentChannel) {
      if (sseCleanup) sseCleanup()
      sseCleanup = connectSSE(`/api/channels/${currentChannel.slug}/stream`, (data) => {
        if (!data || !data.id) return
        // Skip if we already have this message (real or temp with same content)
        if (messages.find(m => m.id === data.id)) return
        // Remove optimistic temp message for this real message
        const tempIdx = messages.findIndex(m => m.id.startsWith('temp-') && m.content === data.content && m.author_id === data.author_id)
        if (tempIdx !== -1) {
          messages[tempIdx] = data // Replace in-place to avoid flash
        } else {
          messages.push(data)
        }
        renderMessagesInPlace()
      }, true)
    }
  }

  function renderMessages() {
    if (!messages.length) return '<p class="text-slate-500 text-center py-8">No messages yet.</p>'

    let html = ''
    let lastDate = ''
    let lastAuthor = ''
    let lastTime = 0

    for (const msg of messages) {
      const dateLabel = formatDateLabel(msg.created_at)
      if (dateLabel !== lastDate) {
        html += `<div class="text-center text-xs text-slate-500 my-3"><span class="bg-slate-900 px-2">${dateLabel}</span></div>`
        lastDate = dateLabel
        lastAuthor = ''
      }

      const msgTime = new Date(msg.created_at).getTime()
      const sameGroup = msg.author_id === lastAuthor && (msgTime - lastTime) < 300000

      if (sameGroup) {
        html += `
          <div class="pl-12 py-0.5 hover:bg-slate-800/30 group relative">
            <span class="absolute right-full mr-1 text-xs text-slate-600 opacity-0 group-hover:opacity-100">${formatTime(msg.created_at)}</span>
            <span class="text-sm text-slate-200">${escapeHTML(msg.content)}</span>
          </div>
        `
      } else {
        html += `
          <div class="flex items-start gap-3 mt-3 hover:bg-slate-800/30 py-1">
            ${avatarHTML({ avatarUrl: msg.author?.avatar_url, size: 36 })}
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <a href="/u/${msg.author?.username || ''}" class="font-semibold text-sm hover:text-indigo-400">${escapeHTML(msg.author?.display_name || msg.author?.username || 'Unknown')}</a>
                <span class="text-xs text-slate-500">${formatTime(msg.created_at)}</span>
              </div>
              <div class="text-sm text-slate-200">${escapeHTML(msg.content)}</div>
            </div>
          </div>
        `
      }

      lastAuthor = msg.author_id
      lastTime = msgTime
    }

    return html
  }

  function renderMessagesInPlace() {
    const area = container.querySelector('#messages-area')
    if (!area) return
    const wasAtBottom = area.scrollHeight - area.scrollTop - area.clientHeight < 50
    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
    area.innerHTML = renderMessages()
    if (wasAtBottom) area.scrollTop = area.scrollHeight
  }

  loadChannels().then(loadMessages).then(render).catch(() => {
    container.innerHTML = `
      <div class="text-center py-8">
        <p class="text-red-400">Failed to load chat.</p>
        <button id="retry-chat" class="mt-2 text-sm text-indigo-400 hover:text-indigo-300">Try again</button>
      </div>
    `
    container.querySelector('#retry-chat')?.addEventListener('click', () => renderChat(container, { channelId }))
  })

  return () => {
    document.body.classList.remove('chat-active')
    if (sseCleanup) sseCleanup()
  }
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
