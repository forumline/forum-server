export function renderNotFound(container) {
  container.innerHTML = `
    <div class="text-center py-16">
      <div class="text-6xl mb-4">:(</div>
      <h1 class="text-2xl font-bold mb-2">Page Not Found</h1>
      <p class="text-slate-400 mb-6">The page you're looking for doesn't exist.</p>
      <a href="/" class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition-colors">Go Home</a>
    </div>
  `
}
