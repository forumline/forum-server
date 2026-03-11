// Forum config — fetched once on boot from /api/config
let config = null

export async function loadConfig() {
  try {
    const res = await fetch('/api/config')
    if (res.ok) {
      config = await res.json()
    }
  } catch {}
  return config
}

export function getConfig() {
  return config || { name: 'Forumline', hosted_mode: false }
}
