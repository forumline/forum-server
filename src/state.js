/*
 * Reactive State Management
 *
 * Provides shared state stores so UI components stay in sync when data changes (e.g., auth status, voice room state).
 *
 * It must:
 * - Allow any component to read and update shared state without tight coupling between modules
 * - Notify all subscribers immediately when state changes so the UI reflects the latest data
 * - Provide a TTL-based cache to avoid redundant API calls for recently fetched data
 */

export function createStore(initial) {
  let state = { ...initial }
  const listeners = new Set()

  return {
    get() {
      return state
    },
    set(updates) {
      state = { ...state, ...updates }
      for (const fn of listeners) fn(state)
    },
    subscribe(fn) {
      listeners.add(fn)
      return () => listeners.delete(fn)
    },
  }
}

/**
 * Simple data cache with TTL.
 */
export function createCache(ttlMs = 30000) {
  const cache = new Map()

  return {
    get(key) {
      const entry = cache.get(key)
      if (!entry) return undefined
      if (Date.now() - entry.time > ttlMs) {
        cache.delete(key)
        return undefined
      }
      return entry.data
    },
    set(key, data) {
      cache.set(key, { data, time: Date.now() })
    },
    invalidate(key) {
      if (key) cache.delete(key)
      else cache.clear()
    },
  }
}
