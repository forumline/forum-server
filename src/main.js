import './style.css'
import { route, setNotFound, resolve, navigate } from './router.js'
import { initAuth, authStore } from './lib/auth.js'
import { initVoice } from './lib/voice.js'
import { renderLayout, getPageContainer } from './components/layout.js'
import { closeMobileSidebar } from './components/sidebar.js'
import { loadConfig, getConfig } from './lib/config.js'

// Pages
import { renderHome } from './pages/home.js'
import { renderLogin } from './pages/login.js'
import { renderRegister } from './pages/register.js'
import { renderCategory } from './pages/category.js'
import { renderThread } from './pages/thread.js'
import { renderNewThread } from './pages/new-thread.js'
import { renderProfile } from './pages/profile.js'
import { renderChat } from './pages/chat.js'
import { renderSearch } from './pages/search.js'
import { renderVoice } from './pages/voice.js'
import { renderBookmarks } from './pages/bookmarks.js'
import { renderSettings } from './pages/settings.js'
import { renderAdmin } from './pages/admin.js'
import { renderNotFound } from './pages/not-found.js'

console.log('[FLD:App] Starting Forumline Demo (vanilla)...')

// Wrap page render to use the layout's page container and close mobile sidebar
function page(renderFn) {
  return (params) => {
    closeMobileSidebar()
    const container = getPageContainer()
    if (!container) return
    return renderFn(container, params)
  }
}

// Auth guard
function requireAuth(renderFn) {
  return (params) => {
    const { user, loading } = authStore.get()
    if (loading) return
    if (!user) {
      const container = getPageContainer()
      if (container) container.innerHTML = '<p class="text-center py-8 text-slate-400"><a href="/login" class="text-indigo-400 hover:text-indigo-300">Sign in</a> to access this page.</p>'
      return
    }
    return page(renderFn)(params)
  }
}

// Redirect if already authenticated
function redirectIfAuth(renderFn) {
  return (params) => {
    const { user, loading } = authStore.get()
    if (loading) return
    if (user) {
      navigate('/', true)
      return
    }
    return page(renderFn)(params)
  }
}

// Register routes
route('/', page(renderHome))
route('/login', redirectIfAuth(renderLogin))
route('/register', redirectIfAuth(renderRegister))
route('/c/:categorySlug', page(renderCategory))
route('/c/:categorySlug/new', requireAuth(renderNewThread))
route('/t/:threadId', page(renderThread))
route('/u/:username', page(renderProfile))
route('/chat', page(renderChat))
route('/chat/:channelId', page(renderChat))
route('/search', page(renderSearch))
route('/voice', page(renderVoice))
route('/voice/:roomId', page(renderVoice))
route('/bookmarks', requireAuth(renderBookmarks))
route('/settings', requireAuth(renderSettings))
route('/admin', page(renderAdmin))
setNotFound(page(renderNotFound))

// Boot
async function boot() {
  // Load forum config (name, hosted mode)
  await loadConfig()
  const cfg = getConfig()
  document.title = cfg.name

  // Render layout shell
  renderLayout(document.getElementById('app'))

  // Initialize auth
  await initAuth()

  // Initialize voice presence
  initVoice()

  // Resolve initial route
  resolve()

  // Re-resolve route when auth state changes (to update auth guards)
  authStore.subscribe(() => {
    // Just re-render the header — the page will re-render on next navigation
  })
}

boot()

// ---- Forumline protocol: respond to parent iframe messages ----
if (window.parent !== window) {
  function sendAuthState() {
    const { user } = authStore.get()
    window.parent.postMessage({ type: 'forumline:auth_state', signedIn: !!user }, '*')
  }

  window.addEventListener('message', (e) => {
    if (e.data?.type === 'forumline:request_auth_state') {
      sendAuthState()
    }
  })

  // Notify parent on auth state changes (login, logout, initial load)
  let lastSignedIn = null
  authStore.subscribe(() => {
    const { loading, user } = authStore.get()
    if (loading) return
    const signedIn = !!user
    if (lastSignedIn === null) {
      window.parent.postMessage({ type: 'forumline:ready' }, '*')
    }
    if (signedIn !== lastSignedIn) {
      lastSignedIn = signedIn
      sendAuthState()
    }
  })
}
