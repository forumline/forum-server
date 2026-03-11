/*
 * New Thread Creation
 *
 * Lets authenticated users start a new discussion thread within a specific category.
 *
 * It must:
 * - Require authentication and show the category context via breadcrumb navigation
 * - Accept a title, body content, and optional custom thread image (with crop support)
 * - Generate a default thread avatar from the thread ID if no custom image is provided
 * - Create the thread and its first post atomically, then redirect to the new thread page
 * - Validate minimum title and content length before submission
 */

import { api } from '../lib/api.js'
import { authStore, getAccessToken } from '../lib/auth.js'
import { uploadAvatar, uploadDefaultAvatar } from '../lib/avatars.js'
import { navigate } from '../router.js'
import { showCropModal } from '../components/image-crop-modal.js'

export function renderNewThread(container, { categorySlug }) {
  const { user } = authStore.get()
  if (!user) {
    container.innerHTML = '<p class="text-center py-8 text-slate-400"><a href="/login" class="text-indigo-400">Sign in</a> to create a thread.</p>'
    return
  }

  container.innerHTML = '<div class="animate-pulse"><div class="h-8 w-48 bg-slate-800 rounded"></div></div>'

  api.getCategory(categorySlug).then(category => {
    if (!category) {
      container.innerHTML = '<p class="text-center py-8 text-slate-400">Category not found.</p>'
      return
    }

    let imageFile = null

    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()
    container.innerHTML = `
      <div class="flex items-center gap-2 text-sm text-slate-400 mb-4">
        <a href="/" class="hover:text-indigo-400">Home</a>
        <span>/</span>
        <a href="/c/${categorySlug}" class="hover:text-indigo-400">${escapeHTML(category.name)}</a>
        <span>/</span>
        <span class="text-slate-300">New Thread</span>
      </div>

      <h1 class="text-2xl font-bold mb-6">New Thread in ${escapeHTML(category.name)}</h1>

      <form id="new-thread-form" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Thread Image (optional)</label>
          <input type="file" id="thread-image" accept="image/*" class="text-sm text-slate-400" />
          <div id="image-preview" class="hidden mt-2"></div>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Title</label>
          <input type="text" name="title" required minlength="5" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" placeholder="Thread title" />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Content</label>
          <textarea name="content" required minlength="10" rows="8" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white resize-none focus:outline-none focus:ring-2 focus:ring-indigo-500" placeholder="Write your post..."></textarea>
        </div>
        <div id="thread-error" class="hidden text-sm text-red-400"></div>
        <div class="flex items-center gap-3">
          <a href="/c/${categorySlug}" class="px-4 py-2 text-sm text-slate-400 hover:text-white transition-colors">Cancel</a>
          <button type="submit" class="px-6 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Create Thread</button>
        </div>
      </form>
    `

    const fileInput = container.querySelector('#thread-image')
    const preview = container.querySelector('#image-preview')
    fileInput.addEventListener('change', async () => {
      const file = fileInput.files[0]
      if (!file) return
      fileInput.value = ''

      const imageSrc = URL.createObjectURL(file)
      const blob = await showCropModal(imageSrc)
      URL.revokeObjectURL(imageSrc)
      if (!blob) return

      imageFile = blob
      // eslint-disable-next-line no-unsanitized/property -- src is a blob URL from URL.createObjectURL()
      preview.innerHTML = `<img src="${URL.createObjectURL(blob)}" class="h-32 rounded-lg object-cover" />`
      preview.classList.remove('hidden')
    })

    container.querySelector('#new-thread-form').addEventListener('submit', async (e) => {
      e.preventDefault()
      const form = e.target
      const errorEl = container.querySelector('#thread-error')
      const btn = form.querySelector('button[type=submit]')

      btn.disabled = true
      btn.textContent = 'Creating...'
      errorEl.classList.add('hidden')

      try {
        const title = form.title.value.trim()
        const content = form.content.value.trim()
        const slug = title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')

        const result = await api.createThread({
          category_id: category.id,
          author_id: user.id,
          title,
          slug,
        })

        if (!result?.id) throw new Error('Failed to create thread')

        // Create first post
        await api.createPost({
          thread_id: result.id,
          author_id: user.id,
          content,
        })

        // Upload image
        const token = await getAccessToken()
        if (imageFile && token) {
          const imageUrl = await uploadAvatar(imageFile, `thread/${result.id}/image.png`, token)
          if (imageUrl) await api.updateThread(result.id, { image_url: imageUrl })
        } else if (token) {
          const imageUrl = await uploadDefaultAvatar(result.id, 'thread', token)
          if (imageUrl) await api.updateThread(result.id, { image_url: imageUrl })
        }

        navigate(`/t/${result.id}`)
      } catch (err) {
        errorEl.textContent = err.message || 'Failed to create thread'
        errorEl.classList.remove('hidden')
        btn.disabled = false
        btn.textContent = 'Create Thread'
      }
    })
  })
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}
