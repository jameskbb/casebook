// Overlay layer stack: menus, dialogs and drawers register here so Esc
// closes only the topmost one (UI-SPEC Overlays stacking), and global
// shortcuts can stand down while any layer is open.

type Layer = { onEscape: () => void }

const stack: Layer[] = []
let listening = false

function onKeyDown(e: KeyboardEvent) {
  if (e.key !== 'Escape' || stack.length === 0) return
  e.preventDefault()
  e.stopPropagation()
  stack[stack.length - 1].onEscape()
}

function ensureListener() {
  if (listening) return
  document.addEventListener('keydown', onKeyDown, true)
  listening = true
}

/**
 * Registers an overlay. onEscape runs when Esc is pressed and this layer is
 * on top. Returns a function that removes the layer.
 */
export function pushLayer(onEscape: () => void): () => void {
  ensureListener()
  const layer: Layer = { onEscape }
  stack.push(layer)
  return () => {
    const i = stack.lastIndexOf(layer)
    if (i >= 0) stack.splice(i, 1)
  }
}

/** True while any menu, dialog or drawer is open. */
export function hasOpenLayer(): boolean {
  return stack.length > 0
}
