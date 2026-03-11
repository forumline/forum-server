/*
 * Forgot Password Page
 *
 * Lets users request a password reset link when they can no longer access their account.
 *
 * It must:
 * - Accept the user's email address and trigger a password reset email
 * - Show a confirmation message after the email is sent so the user knows to check their inbox
 * - Display clear error messages if the reset request fails
 */

import { resetPassword } from '../lib/auth.js'

export function renderForgotPassword(container) {
  container.innerHTML = `
    <div class="max-w-md mx-auto mt-12">
      <h1 class="text-2xl font-bold mb-2">Reset Password</h1>
      <p class="text-slate-400 mb-6">Enter your email and we'll send you a reset link.</p>
      <form id="forgot-form" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Email</label>
          <input type="email" name="email" required class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <div id="forgot-error" class="hidden text-sm text-red-400"></div>
        <button type="submit" class="w-full py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Send Reset Link</button>
      </form>
      <div id="forgot-success" class="hidden text-center">
        <div class="text-green-400 mb-2">Check your email for the reset link.</div>
        <a href="/login" class="text-sm text-indigo-400 hover:text-indigo-300">Back to sign in</a>
      </div>
      <p class="mt-4 text-center text-sm"><a href="/login" class="text-indigo-400 hover:text-indigo-300">Back to sign in</a></p>
    </div>
  `

  container.querySelector('#forgot-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    const form = e.target
    const errorEl = container.querySelector('#forgot-error')
    const btn = form.querySelector('button[type=submit]')

    btn.disabled = true
    btn.textContent = 'Sending...'
    errorEl.classList.add('hidden')

    const { error } = await resetPassword(form.email.value)
    if (error) {
      errorEl.textContent = error.message
      errorEl.classList.remove('hidden')
      btn.disabled = false
      btn.textContent = 'Send Reset Link'
    } else {
      form.classList.add('hidden')
      container.querySelector('#forgot-success').classList.remove('hidden')
    }
  })
}
