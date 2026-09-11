// Wraps a state change in a view transition when the browser supports one.
//
// Without this, switching tabs swaps the DOM instantly and the eye has to
// re-find its place. With it, the browser captures before and after and
// crossfades between them — the same mechanism a native app uses.
//
// The fallback is not a polyfill: an instant swap is exactly what the app did
// before, so unsupported browsers lose nothing.
export function withTransition(update: () => void) {
  const doc = document as Document & {
    startViewTransition?: (cb: () => void) => void
  }
  if (!doc.startViewTransition || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    update()
    return
  }
  doc.startViewTransition(update)
}
