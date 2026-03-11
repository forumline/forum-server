/*
 * Reset Password Page
 *
 * Lets users set a new password after clicking the recovery link in their email.
 *
 * It must:
 * - Verify the recovery token from the URL hash before showing the password form
 * - Show an "invalid or expired link" message if the token cannot be validated
 * - Require password confirmation to prevent typos
 * - Redirect to the home page after a successful password update
 */

import { updatePassword, authStore } from '../lib/auth.js'
import { navigate } from '../router.js'

export function renderResetPassword(container) {
  const { user } = authStore.get()

  // If user is already set, recovery token was processed — show form
  if (user) {
    showForm(container)
    return
  }

  // Wait up to 2s for auth to process recovery token from URL hash
  container.innerHTML = `
    <div class="max-w-md mx-auto mt-12 text-center">
      <div class="animate-pulse"><div class="h-8 w-48 bg-slate-800 rounded mx-auto"></div></div>
      <p class="text-slate-400 mt-4">Verifying reset link...</p>
    </div>
  `

  const timer = setTimeout(() => {
    const { user: u } = authStore.get()
    if (u) {
      showForm(container)
    } else {
      container.innerHTML = `
        <div class="max-w-md mx-auto mt-12">
          <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-8 text-center">
            <svg class="mx-auto h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/></svg>
            <h1 class="text-xl font-bold mt-4">Invalid or Expired Link</h1>
            <p class="text-slate-400 mt-2">This password reset link is invalid or has expired. Please request a new one.</p>
            <a href="/forgot-password" class="inline-block mt-4 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Request New Link</a>
          </div>
        </div>
      `
    }
  }, 2000)

  // Also listen for auth changes in case it resolves faster
  const unsub = authStore.subscribe(() => {
    const { user: u } = authStore.get()
    if (u) {
      clearTimeout(timer)
      unsub()
      showForm(container)
    }
  })

  return () => { clearTimeout(timer); unsub() }
}

function showForm(container) {
  container.innerHTML = `
    <div class="max-w-md mx-auto mt-12">
      <h1 class="text-2xl font-bold mb-6">Set New Password</h1>
      <form id="reset-form" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">New Password</label>
          <input type="password" name="password" required minlength="6" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Confirm Password</label>
          <input type="password" name="confirm" required minlength="6" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <div id="reset-error" class="hidden text-sm text-red-400"></div>
        <button type="submit" class="w-full py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Update Password</button>
      </form>
    </div>
  `

  container.querySelector('#reset-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    const form = e.target
    const errorEl = container.querySelector('#reset-error')

    if (form.password.value !== form.confirm.value) {
      errorEl.textContent = 'Passwords do not match'
      errorEl.classList.remove('hidden')
      return
    }

    const btn = form.querySelector('button[type=submit]')
    btn.disabled = true
    btn.textContent = 'Updating...'
    errorEl.classList.add('hidden')

    const { error } = await updatePassword(form.password.value)
    if (error) {
      errorEl.textContent = error.message
      errorEl.classList.remove('hidden')
      btn.disabled = false
      btn.textContent = 'Update Password'
    } else {
      container.innerHTML = `
        <div class="max-w-md mx-auto mt-12">
          <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-8 text-center">
            <svg class="mx-auto h-12 w-12 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
            <h1 class="text-xl font-bold mt-4">Password Updated</h1>
            <p class="text-slate-400 mt-2">Your password has been successfully reset. Redirecting...</p>
          </div>
        </div>
      `
      setTimeout(() => navigate('/'), 2000)
    }
  })
}
