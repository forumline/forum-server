/*
 * Client-Side Page Router
 *
 * Enables single-page navigation so users move between forum pages instantly without full reloads.
 *
 * It must:
 * - Map URL paths to page handlers so every forum section has a shareable, bookmarkable URL
 * - Intercept in-app link clicks to prevent page reloads and preserve application state
 * - Support browser back/forward buttons so navigation history works as users expect
 * - Clean up the previous page's resources (SSE connections, timers) before rendering a new one
 * - Show a 404 page when users visit a URL that doesn't match any known route
 */

const routes = []
let currentCleanup = null
let notFoundHandler = null
const routeListeners = new Set()

export function route(pattern, handler) {
  const keys = []
  const regex = pattern
    .replace(/:(\w+)/g, (_, key) => {
      keys.push(key)
      return '([^/]+)'
    })
    .replace(/\*/g, '.*')
  routes.push({ pattern, regex: new RegExp(`^${regex}$`), keys, handler })
}

export function setNotFound(handler) {
  notFoundHandler = handler
}

export function navigate(path, replace = false) {
  if (replace) {
    history.replaceState(null, '', path)
  } else {
    history.pushState(null, '', path)
  }
  resolve()
}

export function resolve() {
  const path = location.pathname
  window.scrollTo(0, 0)

  // Clean up previous page
  if (currentCleanup) {
    currentCleanup()
    currentCleanup = null
  }

  for (const { regex, keys, handler } of routes) {
    const match = path.match(regex)
    if (match) {
      const params = {}
      keys.forEach((key, i) => {
        params[key] = decodeURIComponent(match[i + 1])
      })
      currentCleanup = handler(params) || null
      routeListeners.forEach(fn => fn(path))
      return
    }
  }

  if (notFoundHandler) {
    currentCleanup = notFoundHandler() || null
  }
  routeListeners.forEach(fn => fn(location.pathname))
}

export function getCurrentPath() {
  return location.pathname
}

export function onRouteChange(fn) {
  routeListeners.add(fn)
  return () => routeListeners.delete(fn)
}

// Handle browser back/forward
window.addEventListener('popstate', resolve)

// Intercept link clicks for SPA navigation
document.addEventListener('click', (e) => {
  const link = e.target.closest('a[href]')
  if (!link) return
  const href = link.getAttribute('href')
  if (!href || href.startsWith('http') || href.startsWith('#') || href.startsWith('mailto:') || href.startsWith('/api/') || href.startsWith('/auth/')) return
  if (link.target === '_blank') return
  e.preventDefault()
  navigate(href)
})
