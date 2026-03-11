/*
 * Avatar Display Component
 *
 * Renders a user's or thread's avatar image consistently across the entire forum UI.
 *
 * It must:
 * - Display the user's custom or generated avatar at the requested size
 * - Show a generic silhouette fallback when no avatar image is available
 * - Optionally display a globe badge to indicate federated Forumline identity users
 */

export function avatarHTML(opts = {}) {
  const { seed = '', type = 'user', size = 40, avatarUrl = null, showGlobe = false, className = '' } = opts
  const sizeClass = `w-${Math.round(size / 4)} h-${Math.round(size / 4)}`
  const sizeStyle = `width:${size}px;height:${size}px;min-width:${size}px;`

  const fallback = `<div class="rounded-full bg-slate-600 flex items-center justify-center ${className}" style="${sizeStyle}">
    <svg class="w-1/2 h-1/2 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
    </svg>
  </div>`

  const globe = showGlobe ? `<div class="absolute -bottom-0.5 -right-0.5 bg-indigo-600 rounded-full p-0.5">
    <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
    </svg>
  </div>` : ''

  if (avatarUrl) {
    return `<div class="relative inline-flex flex-shrink-0">
      <img src="${avatarUrl}" alt="" class="rounded-full object-cover ${className}" style="${sizeStyle}" />
      ${globe}
    </div>`
  }

  return `<div class="relative inline-flex flex-shrink-0">${fallback}${globe}</div>`
}
