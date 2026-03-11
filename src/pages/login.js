/*
 * Sign In Page
 *
 * Provides the login form so returning users can authenticate and access their forum account.
 *
 * It must:
 * - Show an email/password form for self-hosted forums with local GoTrue auth
 * - Show a "Sign in with Forumline" button for federated identity login (sole option on hosted forums)
 * - Redirect to the home page on successful authentication
 * - Display validation errors when credentials are incorrect
 * - Link to the registration and forgot password pages
 */

import { signIn } from '../lib/auth.js'
import { navigate } from '../router.js'
import { getConfig } from '../lib/config.js'

export function renderLogin(container) {
  const { hosted_mode } = getConfig()

  if (hosted_mode) {
    // Hosted forums have no local auth — only Forumline identity
    container.innerHTML = `
      <div class="max-w-md mx-auto mt-12">
        <h1 class="text-2xl font-bold mb-6">Sign In</h1>
        <a href="/api/forumline/auth" class="block w-full py-3 text-center bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Sign in with Forumline</a>
        <p class="mt-4 text-sm text-slate-400 text-center">Don't have an account? <a href="/register" class="text-indigo-400 hover:text-indigo-300">Sign up on Forumline</a></p>
      </div>
    `
    return
  }

  container.innerHTML = `
    <div class="max-w-md mx-auto mt-12">
      <h1 class="text-2xl font-bold mb-6">Sign In</h1>
      <form id="login-form" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Email</label>
          <input type="email" name="email" required class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Password</label>
          <input type="password" name="password" required class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <div id="login-error" class="hidden text-sm text-red-400"></div>
        <button type="submit" class="w-full py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Sign In</button>
      </form>
      <div class="mt-4 text-center space-y-2">
        <a href="/forgot-password" class="text-sm text-indigo-400 hover:text-indigo-300">Forgot password?</a>
        <p class="text-sm text-slate-400">Don't have an account? <a href="/register" class="text-indigo-400 hover:text-indigo-300">Sign up</a></p>
      </div>
      <div class="mt-6 border-t border-slate-700 pt-6">
        <a href="/api/forumline/auth" class="block w-full py-2 text-center bg-slate-800 hover:bg-slate-700 border border-slate-600 text-white rounded-lg font-medium transition-colors">Sign in with Forumline</a>
      </div>
    </div>
  `

  container.querySelector('#login-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    const form = e.target
    const email = form.email.value
    const password = form.password.value
    const errorEl = container.querySelector('#login-error')
    const btn = form.querySelector('button[type=submit]')

    btn.disabled = true
    btn.textContent = 'Signing in...'
    errorEl.classList.add('hidden')

    const { error } = await signIn(email, password)
    if (error) {
      errorEl.textContent = error.message
      errorEl.classList.remove('hidden')
      btn.disabled = false
      btn.textContent = 'Sign In'
    } else {
      navigate('/')
    }
  })
}
