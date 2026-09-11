import { useEffect, useState } from 'react'

// A frame ticker that respects the two things that make live animation
// acceptable rather than annoying:
//
//   - prefers-reduced-motion: some people get motion sick, and a constantly
//     moving chart is exactly the kind of thing that triggers it.
//   - document visibility: animating a chart on a tab nobody is looking at
//     burns battery for nothing.
//
// Returns a monotonically increasing timestamp, so callers can interpolate
// against wall-clock time rather than counting frames.
export function useTicker(active = true): number {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!active) return
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return

    let frame = 0
    let running = true

    const loop = () => {
      if (!running) return
      setNow(Date.now())
      frame = requestAnimationFrame(loop)
    }

    const onVisibility = () => {
      if (document.hidden) {
        running = false
        cancelAnimationFrame(frame)
      } else if (!running) {
        running = true
        frame = requestAnimationFrame(loop)
      }
    }

    frame = requestAnimationFrame(loop)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      running = false
      cancelAnimationFrame(frame)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [active])

  return now
}
