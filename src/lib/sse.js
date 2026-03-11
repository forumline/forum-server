/*
 * Real-Time Event Stream
 *
 * Keeps the forum UI live-updated by maintaining a persistent server connection for push events (new messages, posts, notifications).
 *
 * It must:
 * - Establish an EventSource connection to the server's SSE endpoint for a given resource
 * - Automatically reconnect after network interruptions so users don't miss updates
 * - Attach the user's auth token when the stream requires authentication
 * - Return a cleanup function so pages can disconnect when the user navigates away
 */

import { getAccessToken } from './auth.js'

export function connectSSE(url, onMessage, requireAuth = false) {
  let es = null
  let reconnectTimer = null
  let cancelled = false

  async function connect() {
    if (cancelled) return

    let fullUrl = url
    if (requireAuth) {
      const token = await getAccessToken()
      if (!token) {
        reconnectTimer = setTimeout(connect, 3000)
        return
      }
      const sep = fullUrl.includes('?') ? '&' : '?'
      fullUrl = `${fullUrl}${sep}access_token=${encodeURIComponent(token)}`
    }

    es = new EventSource(fullUrl)

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        onMessage(data)
      } catch {
        // Ignore heartbeats
      }
    }

    es.onerror = () => {
      if (cancelled) return
      es?.close()
      es = null
      reconnectTimer = setTimeout(connect, 3000)
    }
  }

  connect()

  return () => {
    cancelled = true
    es?.close()
    if (reconnectTimer) clearTimeout(reconnectTimer)
  }
}
