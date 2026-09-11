import type { Terminal } from '@xterm/xterm'

export function attachTerminalTouch(term: Terminal): () => void {
  const element = term.element
  if (!element) return () => {}
  let gesture: { id: number; x: number; y: number; cellHeight: number; lines: number } | undefined
  const reset = () => { gesture = undefined }
  const start = (event: TouchEvent) => {
    reset()
    if (event.touches.length !== 1 || term.buffer.active.type !== 'normal') return
    if (event.target instanceof Element && event.target.closest('.scrollbar')) return
    const cellHeight = element.getBoundingClientRect().height / term.rows
    if (!cellHeight) return
    const touch = event.touches[0]
    gesture = { id: touch.identifier, x: touch.clientX, y: touch.clientY, cellHeight, lines: 0 }
  }
  const move = (event: TouchEvent) => {
    if (!gesture || event.touches.length !== 1 || term.buffer.active.type !== 'normal') { reset(); return }
    const touch = event.touches[0]
    if (touch.identifier !== gesture.id) { reset(); return }
    const deltaY = gesture.y - touch.clientY
    if (Math.abs(deltaY) < 6 || Math.abs(deltaY) < Math.abs(gesture.x - touch.clientX)) return
    if (event.cancelable) event.preventDefault()
    const lines = Math.trunc(deltaY / gesture.cellHeight)
    if (lines !== gesture.lines) term.scrollLines(lines - gesture.lines)
    gesture.lines = lines
  }
  element.addEventListener('touchstart', start, { passive: true })
  element.addEventListener('touchmove', move, { passive: false })
  element.addEventListener('touchend', reset)
  element.addEventListener('touchcancel', reset)
  return () => {
    element.removeEventListener('touchstart', start)
    element.removeEventListener('touchmove', move)
    element.removeEventListener('touchend', reset)
    element.removeEventListener('touchcancel', reset)
  }
}
