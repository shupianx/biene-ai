// Singleton loader for the avatar sprite atlas (public/avatar.png).
//
// Every AgentAvatar instance shares the same Image. The browser caches
// the underlying texture, so even though hundreds of canvases call
// drawImage on it per frame, there's exactly one decode + one upload.

const SPRITE_URL = '/avatar.png'

let cached: HTMLImageElement | null = null
let pending: Promise<HTMLImageElement> | null = null

/**
 * Returns the loaded sprite synchronously, or null if it hasn't finished
 * loading yet. Avatars use this in their render loop and skip drawing
 * until it returns non-null.
 */
export function getAvatarSprite(): HTMLImageElement | null {
  return cached
}

/**
 * Kicks off (or awaits) the one-time sprite load. Idempotent — multiple
 * concurrent callers share the same in-flight Promise.
 */
export function ensureAvatarSprite(): Promise<HTMLImageElement> {
  if (cached) return Promise.resolve(cached)
  if (pending) return pending
  pending = new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      cached = img
      resolve(img)
    }
    img.onerror = () => {
      pending = null
      reject(new Error(`Failed to load avatar sprite: ${SPRITE_URL}`))
    }
    img.src = SPRITE_URL
  })
  return pending
}
