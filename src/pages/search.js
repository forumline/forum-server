/*
 * Search Page
 *
 * Enables users to find threads and posts across the entire forum by keyword.
 *
 * It must:
 * - Provide instant search-as-you-type with debounced API calls
 * - Search both threads and posts simultaneously and show combined or filtered results
 * - Highlight matching text in search results so users can quickly assess relevance
 * - Persist the search query and filter in the URL so results are shareable and survive page refresh
 * - Show suggested search terms when the search field is empty to encourage discovery
 */

import { api } from '../lib/api.js'
import { formatRelativeTime } from '../lib/date.js'

export function renderSearch(container) {
  let debounceTimer = null
  let filter = 'all'
  let currentQuery = ''
  let threadResults = []
  let postResults = []

  // Read initial query from URL
  const params = new URLSearchParams(location.search)
  const initialQuery = params.get('q') || ''
  const initialFilter = params.get('filter') || 'all'
  filter = initialFilter

  // eslint-disable-next-line no-unsanitized/property -- static template, query escaped via escapeAttr()
  container.innerHTML = `
    <h1 class="text-2xl font-bold mb-6">Search</h1>
    <div class="relative mb-4">
      <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
      <input id="search-input" type="text" placeholder="Search threads and posts..." value="${escapeAttr(initialQuery)}" class="w-full pl-10 pr-10 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" autofocus />
      <button id="search-clear" class="${initialQuery ? '' : 'hidden '}absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
      </button>
    </div>
    <div id="search-filters" class="hidden flex gap-2 mb-4">
      <button class="filter-btn px-3 py-1 rounded-lg text-sm ${filter === 'all' ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300'}" data-filter="all">All</button>
      <button class="filter-btn px-3 py-1 rounded-lg text-sm ${filter === 'threads' ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300'}" data-filter="threads">Threads <span id="thread-count"></span></button>
      <button class="filter-btn px-3 py-1 rounded-lg text-sm ${filter === 'posts' ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300'}" data-filter="posts">Posts <span id="post-count"></span></button>
    </div>
    <div id="search-results">
      ${!initialQuery ? emptyStateHTML() : ''}
    </div>
  `

  const input = container.querySelector('#search-input')
  const clearBtn = container.querySelector('#search-clear')
  const filtersEl = container.querySelector('#search-filters')

  // Run initial search if query in URL
  if (initialQuery) doSearch(initialQuery)

  function updateURL(q) {
    const p = new URLSearchParams()
    if (q) { p.set('q', q); p.set('filter', filter) }
    const newUrl = p.toString() ? `${location.pathname}?${p}` : location.pathname
    history.replaceState(null, '', newUrl)
  }

  input.addEventListener('input', () => {
    const q = input.value.trim()
    clearBtn.classList.toggle('hidden', !q)

    clearTimeout(debounceTimer)
    if (!q) {
      currentQuery = ''
      updateURL('')
      // eslint-disable-next-line no-unsanitized/property -- static empty state template
      container.querySelector('#search-results').innerHTML = emptyStateHTML()
      filtersEl.classList.add('hidden')
      return
    }

    debounceTimer = setTimeout(() => doSearch(q), 300)
  })

  clearBtn.addEventListener('click', () => {
    input.value = ''
    currentQuery = ''
    updateURL('')
    clearBtn.classList.add('hidden')
    // eslint-disable-next-line no-unsanitized/property -- static empty state template
    container.querySelector('#search-results').innerHTML = emptyStateHTML()
    filtersEl.classList.add('hidden')
    input.focus()
  })

  // Suggested search term clicks
  container.addEventListener('click', (e) => {
    const suggBtn = e.target.closest('.suggest-btn')
    if (suggBtn) {
      input.value = suggBtn.dataset.term
      clearBtn.classList.remove('hidden')
      doSearch(suggBtn.dataset.term)
    }
  })

  container.querySelectorAll('.filter-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      filter = btn.dataset.filter
      updateURL(currentQuery)
      container.querySelectorAll('.filter-btn').forEach(b => {
        b.className = `filter-btn px-3 py-1 rounded-lg text-sm ${b.dataset.filter === filter ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300'}`
      })
      renderResults()
    })
  })

  async function doSearch(query) {
    currentQuery = query
    updateURL(query)
    container.querySelector('#search-results').innerHTML = '<div class="animate-pulse"><div class="h-16 bg-slate-800 rounded-xl"></div></div>'
    try {
      [threadResults, postResults] = await Promise.all([
        api.searchThreads(query).catch(() => []),
        api.searchPosts(query).catch(() => []),
      ])

      filtersEl.classList.remove('hidden')
      container.querySelector('#thread-count').textContent = `(${threadResults.length})`
      container.querySelector('#post-count').textContent = `(${postResults.length})`
      renderResults()
    } catch {
      container.querySelector('#search-results').innerHTML = `
        <div class="text-center py-8">
          <p class="text-red-400">Failed to search.</p>
          <button class="mt-2 text-sm text-indigo-400 hover:text-indigo-300" onclick="this.closest('[id=search-results]').innerHTML=''">Try again</button>
        </div>
      `
    }
  }

  function renderResults() {
    const resultsEl = container.querySelector('#search-results')
    const threads = filter !== 'posts' ? threadResults : []
    const posts = filter !== 'threads' ? postResults : []

    if (!threads.length && !posts.length) {
      // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
      resultsEl.innerHTML = `
        <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-8 text-center">
          <h3 class="text-lg font-medium text-white">No results for "${escapeHTML(currentQuery)}"</h3>
          <p class="mt-2 text-slate-400">Try different keywords or check your spelling.</p>
        </div>
      `
      return
    }

    let html = ''
    if (threads.length) {
      html += threads.map(t => `
        <a href="/t/${t.id}" class="block bg-slate-800/50 border border-slate-700/50 rounded-xl p-4 hover:bg-slate-800 transition-colors mb-2">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-xs bg-indigo-600/30 text-indigo-400 px-1.5 py-0.5 rounded">Thread</span>
            <h3 class="font-semibold text-white">${highlightMatch(escapeHTML(t.title), currentQuery)}</h3>
          </div>
          <div class="text-xs text-slate-400">${escapeHTML(t.author?.display_name || '')} &middot; ${formatRelativeTime(t.created_at)} &middot; ${t.post_count || 0} replies</div>
        </a>
      `).join('')
    }
    if (posts.length) {
      html += posts.map(p => `
        <a href="/t/${p.thread_id}#post-${p.id}" class="block bg-slate-800/50 border border-slate-700/50 rounded-xl p-4 hover:bg-slate-800 transition-colors mb-2">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-xs bg-green-600/30 text-green-400 px-1.5 py-0.5 rounded">Post</span>
            <span class="text-sm font-medium">${escapeHTML(p.author?.display_name || '')}</span>
            <span class="text-xs text-slate-500">${formatRelativeTime(p.created_at)}</span>
          </div>
          <div class="text-sm text-slate-300 line-clamp-2">${highlightMatch(escapeHTML(p.content), currentQuery)}</div>
        </a>
      `).join('')
    }
    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
    resultsEl.innerHTML = html
  }
}

function emptyStateHTML() {
  return `
    <div class="bg-slate-800/50 border border-slate-700/50 rounded-xl p-8 text-center">
      <svg class="mx-auto h-12 w-12 text-slate-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
      <h3 class="mt-4 text-lg font-medium text-white">Search the forum</h3>
      <p class="mt-2 text-slate-400">Start typing to see results instantly.</p>
      <div class="mt-4">
        <p class="mb-2 text-xs text-slate-500">Try searching for:</p>
        <div class="flex flex-wrap justify-center gap-2">
          ${['welcome', 'voice', 'features', 'admin'].map(t => `<button class="suggest-btn rounded-lg bg-slate-700 px-3 py-1.5 text-sm text-slate-300 hover:bg-slate-600 transition-colors" data-term="${t}">${t}</button>`).join('')}
        </div>
      </div>
    </div>
  `
}

function highlightMatch(text, query) {
  if (!query.trim()) return text
  try {
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const regex = new RegExp(`(${escaped})`, 'gi')
    return text.replace(regex, '<mark class="bg-indigo-500/30 text-white rounded px-0.5">$1</mark>')
  } catch {
    return text
  }
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}

function escapeAttr(str) {
  return (str || '').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
