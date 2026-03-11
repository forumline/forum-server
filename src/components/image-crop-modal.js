/*
 * Image Crop Modal
 *
 * Lets users crop and zoom an image into a square before uploading it as an avatar or thread image.
 *
 * It must:
 * - Show a circular crop preview so users can position their image exactly how they want
 * - Support drag-to-pan and zoom slider for precise framing on both desktop and mobile
 * - Export the cropped area as a 256x256 PNG blob ready for upload
 * - Allow cancellation without making any changes
 */

export function showCropModal(imageSrc) {
  return new Promise((resolve) => {
    let crop = { x: 0, y: 0 }
    let zoom = 1
    let dragging = false
    let dragStart = { x: 0, y: 0 }
    let img = null
    let containerSize = 288

    const overlay = document.createElement('div')
    overlay.className = 'fixed inset-0 z-50 flex items-center justify-center bg-black/70'
    // eslint-disable-next-line no-unsanitized/property -- static template, imageSrc is a blob URL from URL.createObjectURL()
    overlay.innerHTML = `
      <div class="bg-slate-800 rounded-xl p-6 w-[340px] shadow-2xl" id="crop-dialog">
        <h3 class="text-lg font-semibold mb-4">Crop Avatar</h3>
        <div class="relative w-72 h-72 mx-auto overflow-hidden rounded-full bg-slate-900 cursor-grab" id="crop-area">
          <img id="crop-img" src="${imageSrc}" class="absolute select-none pointer-events-none" draggable="false" />
        </div>
        <div class="mt-4 flex items-center gap-3">
          <span class="text-xs text-slate-400">Zoom</span>
          <input type="range" id="crop-zoom" min="1" max="3" step="0.05" value="1" class="flex-1" />
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <button id="crop-cancel" class="px-4 py-2 text-sm text-slate-400 hover:text-white transition-colors">Cancel</button>
          <button id="crop-save" class="px-4 py-2 text-sm bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-medium transition-colors">Save</button>
        </div>
      </div>
    `

    document.body.appendChild(overlay)

    const cropArea = overlay.querySelector('#crop-area')
    const cropImg = overlay.querySelector('#crop-img')
    const zoomInput = overlay.querySelector('#crop-zoom')

    // Load image to get dimensions
    img = new Image()
    img.onload = () => {
      updateImageTransform()
    }
    img.src = imageSrc

    function updateImageTransform() {
      if (!img.naturalWidth) return
      // Scale image so shortest side fills the container at current zoom
      const aspect = img.naturalWidth / img.naturalHeight
      let w, h
      if (aspect >= 1) {
        h = containerSize * zoom
        w = h * aspect
      } else {
        w = containerSize * zoom
        h = w / aspect
      }

      // Clamp crop position so image covers the circle
      const maxX = 0
      const minX = containerSize - w
      const maxY = 0
      const minY = containerSize - h
      crop.x = Math.min(maxX, Math.max(minX, crop.x))
      crop.y = Math.min(maxY, Math.max(minY, crop.y))

      cropImg.style.width = w + 'px'
      cropImg.style.height = h + 'px'
      cropImg.style.left = crop.x + 'px'
      cropImg.style.top = crop.y + 'px'
    }

    // Drag to pan
    cropArea.addEventListener('mousedown', (e) => {
      dragging = true
      dragStart = { x: e.clientX - crop.x, y: e.clientY - crop.y }
      cropArea.style.cursor = 'grabbing'
    })
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)

    // Touch support
    cropArea.addEventListener('touchstart', (e) => {
      if (e.touches.length === 1) {
        dragging = true
        dragStart = { x: e.touches[0].clientX - crop.x, y: e.touches[0].clientY - crop.y }
      }
    })
    cropArea.addEventListener('touchmove', (e) => {
      if (dragging && e.touches.length === 1) {
        e.preventDefault()
        crop.x = e.touches[0].clientX - dragStart.x
        crop.y = e.touches[0].clientY - dragStart.y
        updateImageTransform()
      }
    }, { passive: false })
    cropArea.addEventListener('touchend', () => { dragging = false })

    function onMouseMove(e) {
      if (!dragging) return
      crop.x = e.clientX - dragStart.x
      crop.y = e.clientY - dragStart.y
      updateImageTransform()
    }

    function onMouseUp() {
      dragging = false
      cropArea.style.cursor = 'grab'
    }

    // Zoom
    zoomInput.addEventListener('input', () => {
      const oldZoom = zoom
      zoom = parseFloat(zoomInput.value)
      // Keep center stable when zooming
      const cx = containerSize / 2
      const cy = containerSize / 2
      crop.x = cx - (cx - crop.x) * (zoom / oldZoom)
      crop.y = cy - (cy - crop.y) * (zoom / oldZoom)
      updateImageTransform()
    })

    // Cancel
    function cleanup() {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      document.removeEventListener('keydown', onKeyDown)
      overlay.remove()
    }

    overlay.querySelector('#crop-cancel').addEventListener('click', () => {
      cleanup()
      resolve(null)
    })

    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) {
        cleanup()
        resolve(null)
      }
    })

    function onKeyDown(e) {
      if (e.key === 'Escape') {
        cleanup()
        resolve(null)
      }
    }
    document.addEventListener('keydown', onKeyDown)

    // Save — crop to canvas
    overlay.querySelector('#crop-save').addEventListener('click', () => {
      if (!img.naturalWidth) return

      const aspect = img.naturalWidth / img.naturalHeight
      let displayW, displayH
      if (aspect >= 1) {
        displayH = containerSize * zoom
        displayW = displayH * aspect
      } else {
        displayW = containerSize * zoom
        displayH = displayW / aspect
      }

      // Calculate source crop in original image coordinates
      const scaleX = img.naturalWidth / displayW
      const scaleY = img.naturalHeight / displayH
      const sx = -crop.x * scaleX
      const sy = -crop.y * scaleY
      const sw = containerSize * scaleX
      const sh = containerSize * scaleY

      const canvas = document.createElement('canvas')
      canvas.width = 256
      canvas.height = 256
      const ctx = canvas.getContext('2d')
      ctx.drawImage(img, sx, sy, sw, sh, 0, 0, 256, 256)

      canvas.toBlob((blob) => {
        cleanup()
        resolve(blob)
      }, 'image/png')
    })
  })
}
