/*
 * Sign In Page
 *
 * Redirects users to Forumline identity for authentication.
 *
 * It must:
 * - Show a "Sign in with Forumline" button as the sole authentication option
 * - Link to Forumline for account creation
 */

export function renderLogin(container) {
  container.innerHTML = `
    <div class="max-w-md mx-auto mt-12">
      <h1 class="text-2xl font-bold mb-6">Sign In</h1>
      <a href="/api/forumline/auth" class="block w-full py-3 text-center bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Sign in with Forumline</a>
      <p class="mt-4 text-sm text-slate-400 text-center">Don't have an account? <a href="/api/forumline/auth" class="text-indigo-400 hover:text-indigo-300">Sign up on Forumline</a></p>
    </div>
  `
}
