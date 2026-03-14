/*
 * Registration Page
 *
 * Redirects users to Forumline for account creation.
 */

export function renderRegister(container) {
  container.innerHTML = `
    <div class="max-w-md mx-auto mt-12">
      <h1 class="text-2xl font-bold mb-6">Create Account</h1>
      <a href="/api/forumline/auth" class="block w-full py-3 text-center bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Sign up with Forumline</a>
      <p class="mt-4 text-sm text-slate-400 text-center">Already have an account? <a href="/login" class="text-indigo-400 hover:text-indigo-300">Sign in</a></p>
    </div>
  `
}
