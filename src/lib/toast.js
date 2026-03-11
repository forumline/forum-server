/*
 * Toast Notifications
 *
 * Displays brief, non-blocking feedback messages so users know when actions succeed or fail without interrupting their workflow.
 *
 * It must:
 * - Show success, error, and informational messages as temporary overlays
 * - Auto-dismiss after a few seconds so notifications don't require manual interaction
 * - Animate in and out smoothly so the experience feels polished
 */

let toastId = 0

export function toast(message, type = 'info') {
  const container = document.getElementById('toast-container')
  if (!container) return

  const id = ++toastId
  const el = document.createElement('div')
  el.id = `toast-${id}`
  el.className = `px-4 py-3 rounded-lg shadow-lg text-sm font-medium transform transition-all duration-300 translate-x-full ${
    type === 'error' ? 'bg-red-600 text-white' :
    type === 'success' ? 'bg-green-600 text-white' :
    'bg-slate-700 text-slate-100'
  }`
  el.textContent = message
  container.appendChild(el)

  // Animate in
  requestAnimationFrame(() => {
    el.classList.remove('translate-x-full')
  })

  // Auto remove
  setTimeout(() => {
    el.classList.add('translate-x-full')
    setTimeout(() => el.remove(), 300)
  }, 3000)
}

toast.success = (msg) => toast(msg, 'success')
toast.error = (msg) => toast(msg, 'error')
