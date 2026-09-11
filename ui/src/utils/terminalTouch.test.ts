import type { Terminal } from '@xterm/xterm'
import { beforeEach, expect, it, vi } from 'vitest'
import { attachTerminalTouch } from './terminalTouch'

let element: HTMLDivElement
let terminal: { element: HTMLDivElement; rows: number; buffer: { active: { type: string } }; scrollLines: ReturnType<typeof vi.fn> }
const touch = (y: number, x = 20, identifier = 1) => ({ clientX: x, clientY: y, identifier })
function dispatch(type: string, touches: ReturnType<typeof touch>[], target: HTMLElement = element) {
  const event = new Event(type, { bubbles: true, cancelable: true })
  Object.defineProperty(event, 'touches', { value: touches })
  target.dispatchEvent(event)
  return event
}
beforeEach(() => {
  element = document.createElement('div')
  element.getBoundingClientRect = () => ({ height: 200 }) as DOMRect
  terminal = { element, rows: 10, buffer: { active: { type: 'normal' } }, scrollLines: vi.fn() }
})

it('maps vertical dragging to terminal rows and accumulates fractional movement', () => {
  const dispose = attachTerminalTouch(terminal as unknown as Terminal)
  dispatch('touchstart', [touch(100)])
  expect(dispatch('touchmove', [touch(109)]).defaultPrevented).toBe(true)
  expect(terminal.scrollLines).not.toHaveBeenCalled()
  dispatch('touchmove', [touch(129)])
  dispatch('touchmove', [touch(145)])
  dispatch('touchmove', [touch(115)])
  expect(terminal.scrollLines.mock.calls).toEqual([[-1], [-1], [2]])
  dispose()
})

it('leaves taps, horizontal swipes and multi-touch gestures alone', () => {
  const dispose = attachTerminalTouch(terminal as unknown as Terminal)
  dispatch('touchstart', [touch(100)])
  expect(dispatch('touchmove', [touch(103)]).defaultPrevented).toBe(false)
  expect(dispatch('touchmove', [touch(110, 100)]).defaultPrevented).toBe(false)
  expect(dispatch('touchmove', [touch(140), touch(140, 40, 2)]).defaultPrevented).toBe(false)
  dispatch('touchmove', [touch(160)])
  dispatch('touchstart', [touch(100)])
  dispatch('touchmove', [touch(160, 20, 2)])
  expect(terminal.scrollLines).not.toHaveBeenCalled()
  dispose()
})

it('does not intercept alternate-screen applications or scrollbar controls', () => {
  const dispose = attachTerminalTouch(terminal as unknown as Terminal)
  terminal.buffer.active.type = 'alternate'
  dispatch('touchstart', [touch(100)])
  expect(dispatch('touchmove', [touch(160)]).defaultPrevented).toBe(false)
  terminal.buffer.active.type = 'normal'
  const scrollbar = document.createElement('div')
  scrollbar.className = 'scrollbar'
  element.append(scrollbar)
  dispatch('touchstart', [touch(100)], scrollbar)
  dispatch('touchmove', [touch(160)], scrollbar)
  expect(terminal.scrollLines).not.toHaveBeenCalled()
  dispose()
})

it('ends gestures on cancellation and removes every listener on disposal', () => {
  const dispose = attachTerminalTouch(terminal as unknown as Terminal)
  for (const end of ['touchend', 'touchcancel']) {
    dispatch('touchstart', [touch(100)])
    dispatch(end, [])
    dispatch('touchmove', [touch(160)])
  }
  expect(terminal.scrollLines).not.toHaveBeenCalled()
  const remove = vi.spyOn(element, 'removeEventListener')
  dispose()
  expect(remove.mock.calls.map(([type]) => type)).toEqual(['touchstart', 'touchmove', 'touchend', 'touchcancel'])
  dispatch('touchstart', [touch(100)])
  dispatch('touchmove', [touch(160)])
  expect(terminal.scrollLines).not.toHaveBeenCalled()
})
