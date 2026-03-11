/*
 * Date and Time Formatting
 *
 * Converts raw timestamps into human-readable labels so users see friendly times like "5m ago" or "Yesterday" throughout the forum.
 *
 * It must:
 * - Show relative time (e.g., "just now", "3h ago", "2d ago") for recent activity to convey recency at a glance
 * - Fall back to absolute dates for older content so timestamps remain meaningful
 * - Provide day-boundary labels ("Today", "Yesterday") for chat message grouping
 */

function toDate(date) {
  return typeof date === 'string' ? new Date(date) : date
}

export function formatRelativeTime(date, opts) {
  const diff = Date.now() - toDate(date).getTime()
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (opts?.short) {
    if (minutes < 1) return 'now'
    if (minutes < 60) return `${minutes}m`
    if (hours < 24) return `${hours}h`
    if (days < 7) return `${days}d`
    return toDate(date).toLocaleDateString()
  }

  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  if (hours < 24) return `${hours}h ago`
  return `${days}d ago`
}

export function formatTime(date) {
  return toDate(date).toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
  })
}

export function formatDate(date) {
  return new Date(date).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function formatDateLabel(date) {
  const d = new Date(date)
  const today = new Date()
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)

  if (d.toDateString() === today.toDateString()) return 'Today'
  if (d.toDateString() === yesterday.toDateString()) return 'Yesterday'
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}
