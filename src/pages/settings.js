import { authStore, getAccessToken, updatePassword } from '../lib/auth.js'
import { api } from '../lib/api.js'
import { uploadAvatar } from '../lib/avatars.js'
import { toast } from '../lib/toast.js'
import { showCropModal } from '../components/image-crop-modal.js'

export function renderSettings(container) {
  const { user, profile } = authStore.get()
  if (!user) {
    container.innerHTML = '<p class="text-center py-8 text-slate-400"><a href="/login" class="text-indigo-400">Sign in</a> to access settings.</p>'
    return
  }

  let tab = 'profile'
  let formData = {
    display_name: profile?.display_name || '',
    bio: profile?.bio || '',
    website: profile?.website || '',
    avatar_url: profile?.avatar_url || '',
  }

  function render() {
    // eslint-disable-next-line no-unsanitized/property -- static template
    container.innerHTML = `
      <h1 class="text-2xl font-bold mb-6">Settings</h1>

      <div class="flex gap-2 mb-6 overflow-x-auto">
        ${['profile', 'account', 'notifications', 'appearance'].map(t => `
          <button class="tab-btn px-4 py-2 rounded-lg text-sm font-medium whitespace-nowrap transition-colors ${t === tab ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}" data-tab="${t}">${t.charAt(0).toUpperCase() + t.slice(1)}</button>
        `).join('')}
      </div>

      <div id="tab-content"></div>
    `

    container.querySelectorAll('.tab-btn').forEach(btn => {
      btn.addEventListener('click', () => { tab = btn.dataset.tab; render() })
    })

    const content = container.querySelector('#tab-content')
    if (tab === 'profile') renderProfileTab(content)
    else if (tab === 'account') renderAccountTab(content)
    else if (tab === 'notifications') renderNotificationsTab(content)
    else if (tab === 'appearance') renderAppearanceTab(content)
  }

  function renderProfileTab(el) {
    // eslint-disable-next-line no-unsanitized/property -- user content escaped via escapeHTML()/escapeAttr()
    el.innerHTML = `
      <div class="space-y-4 max-w-lg">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-2">Avatar</label>
          <div class="flex items-center gap-3">
            <img src="${formData.avatar_url || ''}" alt="" class="w-16 h-16 rounded-full object-cover bg-slate-700" />
            <input type="file" id="avatar-file" accept="image/*" class="text-sm text-slate-400" />
          </div>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Display Name</label>
          <input type="text" id="display-name" value="${escapeAttr(formData.display_name)}" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Bio</label>
          <textarea id="bio" rows="3" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white resize-none focus:outline-none focus:ring-2 focus:ring-indigo-500">${escapeHTML(formData.bio)}</textarea>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Website</label>
          <input type="url" id="website" value="${escapeAttr(formData.website)}" placeholder="https://" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <button id="save-profile" class="px-6 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Save</button>
      </div>
    `

    const fileInput = el.querySelector('#avatar-file')
    fileInput.addEventListener('change', async () => {
      const file = fileInput.files[0]
      if (!file) return
      fileInput.value = ''

      const imageSrc = URL.createObjectURL(file)
      const blob = await showCropModal(imageSrc)
      URL.revokeObjectURL(imageSrc)
      if (!blob) return

      const token = await getAccessToken()
      if (!token) return
      const url = await uploadAvatar(blob, `user/${user.id}/avatar.png`, token)
      if (url) {
        formData.avatar_url = url
        el.querySelector('img').src = url
      }
    })

    el.querySelector('#save-profile').addEventListener('click', async () => {
      const btn = el.querySelector('#save-profile')
      btn.disabled = true
      btn.textContent = 'Saving...'

      formData.display_name = el.querySelector('#display-name').value
      formData.bio = el.querySelector('#bio').value
      formData.website = el.querySelector('#website').value

      try {
        await api.updateProfile(user.id, formData)
        toast.success('Profile updated')
        // Update local auth store
        const newProfile = await api.getProfile(user.id)
        if (newProfile) {
          authStore.set({
            profile: newProfile,
            user: { ...user, username: newProfile.username, avatar: newProfile.avatar_url },
          })
        }
      } catch { toast.error('Failed to save profile') }
      btn.disabled = false
      btn.textContent = 'Save'
    })
  }

  function renderAccountTab(el) {
    // eslint-disable-next-line no-unsanitized/property -- static template
    el.innerHTML = `
      <div class="space-y-6 max-w-lg">
        <div>
          <h3 class="font-semibold mb-3">Change Password</h3>
          <div class="space-y-3">
            <input type="password" id="new-password" placeholder="New password" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
            <input type="password" id="confirm-password" placeholder="Confirm password" class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-indigo-500" />
            <div id="password-error" class="hidden text-sm text-red-400"></div>
            <button id="change-password" class="px-6 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Update Password</button>
          </div>
        </div>

        <div class="border-t border-slate-700 pt-6">
          <h3 class="font-semibold mb-3">Forumline Connection</h3>
          ${profile?.forumline_id ? `
            <div class="flex items-center gap-2 text-sm">
              <span class="text-green-400">Connected</span>
              <button id="disconnect-forumline" class="text-red-400 hover:text-red-300 text-sm">Disconnect</button>
            </div>
          ` : `
            <a href="/api/forumline/auth" class="inline-block px-4 py-2 bg-slate-800 border border-slate-600 hover:bg-slate-700 text-white rounded-lg text-sm transition-colors">Connect Forumline Account</a>
          `}
        </div>
      </div>
    `

    el.querySelector('#change-password')?.addEventListener('click', async () => {
      const newPw = el.querySelector('#new-password').value
      const confirmPw = el.querySelector('#confirm-password').value
      const errorEl = el.querySelector('#password-error')

      if (newPw !== confirmPw) {
        errorEl.textContent = 'Passwords do not match'
        errorEl.classList.remove('hidden')
        return
      }
      if (newPw.length < 6) {
        errorEl.textContent = 'Password must be at least 6 characters'
        errorEl.classList.remove('hidden')
        return
      }

      const { error } = await updatePassword(newPw)
      if (error) {
        errorEl.textContent = error.message
        errorEl.classList.remove('hidden')
      } else {
        toast.success('Password updated')
        el.querySelector('#new-password').value = ''
        el.querySelector('#confirm-password').value = ''
        errorEl.classList.add('hidden')
      }
    })

    el.querySelector('#disconnect-forumline')?.addEventListener('click', async () => {
      try {
        await fetch(`/api/profiles/${user.id}/forumline-id`, { method: 'DELETE', headers: { 'Authorization': `Bearer ${await getAccessToken()}` } })
        toast.success('Forumline disconnected')
        render()
      } catch { toast.error('Failed to disconnect') }
    })
  }

  function renderNotificationsTab(el) {
    el.innerHTML = '<div class="animate-pulse"><div class="h-40 bg-slate-800 rounded-xl"></div></div>'

    api.getNotificationPreferences().then(prefs => {
      const categories = [
        { key: 'thread_replies', label: 'Replies to your threads' },
        { key: 'post_mentions', label: 'Mentions in posts' },
        { key: 'chat_mentions', label: 'Mentions in chat' },
        { key: 'direct_messages', label: 'Direct messages' },
      ]

      // eslint-disable-next-line no-unsanitized/property -- static template with trusted labels
      el.innerHTML = `
        <div class="space-y-4 max-w-lg">
          ${categories.map(cat => {
            const pref = prefs?.find(p => p.category === cat.key)
            const enabled = pref?.enabled ?? true
            return `
              <div class="flex items-center justify-between py-2">
                <span class="text-sm">${cat.label}</span>
                <button class="notif-toggle w-10 h-6 rounded-full transition-colors ${enabled ? 'bg-indigo-600' : 'bg-slate-700'}" data-category="${cat.key}" data-enabled="${enabled}">
                  <div class="w-4 h-4 bg-white rounded-full transform transition-transform ${enabled ? 'translate-x-5' : 'translate-x-1'}"></div>
                </button>
              </div>
            `
          }).join('')}
        </div>
      `

      el.querySelectorAll('.notif-toggle').forEach(btn => {
        btn.addEventListener('click', async () => {
          const enabled = btn.dataset.enabled === 'true'
          const newEnabled = !enabled
          btn.dataset.enabled = String(newEnabled)
          btn.className = `notif-toggle w-10 h-6 rounded-full transition-colors ${newEnabled ? 'bg-indigo-600' : 'bg-slate-700'}`
          btn.querySelector('div').className = `w-4 h-4 bg-white rounded-full transform transition-transform ${newEnabled ? 'translate-x-5' : 'translate-x-1'}`
          try {
            await api.updateNotificationPreference(btn.dataset.category, newEnabled)
          } catch { toast.error('Failed to update preference') }
        })
      })
    }).catch(() => { el.innerHTML = '<p class="text-red-400">Failed to load preferences.</p>' })
  }

  function renderAppearanceTab(el) {
    const theme = localStorage.getItem('theme') || 'dark'
    const fontSize = localStorage.getItem('fontSize') || 'medium'

    // eslint-disable-next-line no-unsanitized/property -- static template
    el.innerHTML = `
      <div class="space-y-6 max-w-lg">
        <div>
          <h3 class="font-semibold mb-3">Theme</h3>
          <div class="flex gap-2">
            ${['dark', 'light', 'system'].map(t => `
              <button class="theme-btn px-4 py-2 rounded-lg text-sm font-medium transition-colors ${t === theme ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}" data-theme="${t}">${t.charAt(0).toUpperCase() + t.slice(1)}</button>
            `).join('')}
          </div>
        </div>
        <div>
          <h3 class="font-semibold mb-3">Font Size</h3>
          <div class="flex gap-2">
            ${['small', 'medium', 'large'].map(s => `
              <button class="size-btn px-4 py-2 rounded-lg text-sm font-medium transition-colors ${s === fontSize ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}" data-size="${s}">${s.charAt(0).toUpperCase() + s.slice(1)}</button>
            `).join('')}
          </div>
        </div>
      </div>
    `

    el.querySelectorAll('.theme-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        localStorage.setItem('theme', btn.dataset.theme)
        el.querySelectorAll('.theme-btn').forEach(b => {
          b.className = `theme-btn px-4 py-2 rounded-lg text-sm font-medium transition-colors ${b.dataset.theme === btn.dataset.theme ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}`
        })
      })
    })

    el.querySelectorAll('.size-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        localStorage.setItem('fontSize', btn.dataset.size)
        el.querySelectorAll('.size-btn').forEach(b => {
          b.className = `size-btn px-4 py-2 rounded-lg text-sm font-medium transition-colors ${b.dataset.size === btn.dataset.size ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'}`
        })
      })
    })
  }

  render()
}

function escapeHTML(str) {
  const div = document.createElement('div')
  div.textContent = str || ''
  return div.innerHTML
}

function escapeAttr(str) {
  return (str || '').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
