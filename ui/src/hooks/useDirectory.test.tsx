import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import { useDirectory } from './useDirectory'
import * as filesApi from '../api/files'

vi.mock('../api/files', () => ({ listFiles: vi.fn() }))
const listFiles = vi.mocked(filesApi.listFiles)
function deferred() {
  let resolve!: (value: any) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<any>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
const entry = (name: string) => ({ name, path: `/${name}`, isDir: true, mode: '', modTime: '', size: 0 })
beforeEach(() => listFiles.mockReset())

it('aborts old requests and ignores out-of-order responses and finally blocks', async () => {
  const first = deferred()
  const second = deferred()
  listFiles.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
  const { result } = renderHook(() => useDirectory(true, 1))
  const signal = listFiles.mock.calls[0][2]!
  act(() => { void result.current.loadFiles('/new') })
  expect(signal.aborted).toBe(true)
  await act(async () => first.resolve({ data: [entry('old')] }))
  expect(result.current.files).toEqual([])
  expect(result.current.loading).toBe(true)
  await act(async () => second.resolve({ data: [entry('new')] }))
  expect(result.current.files[0].name).toBe('new')
  expect(result.current.currentPath).toBe('/new')
  expect(result.current.ready).toBe(true)
})

it('invalidates callbacks when the modal closes or changes server', async () => {
  const first = deferred()
  const second = deferred()
  listFiles.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
  const { result, rerender, unmount } = renderHook(({ open, server }) => useDirectory(open, server), { initialProps: { open: true, server: 1 } })
  const firstSignal = listFiles.mock.calls[0][2]!
  rerender({ open: true, server: 2 })
  expect(firstSignal.aborted).toBe(true)
  await act(async () => first.resolve({ data: [entry('server-one')] }))
  expect(result.current.files).toEqual([])
  const secondSignal = listFiles.mock.calls[1][2]!
  rerender({ open: false, server: 2 })
  expect(secondSignal.aborted).toBe(true)
  await act(async () => second.resolve({ data: [entry('server-two')] }))
  expect(result.current.files).toEqual([])
  expect(result.current.ready).toBe(false)
  unmount()
})

it('refreshes mutations only while their original server and directory are still visible', async () => {
  listFiles.mockResolvedValue({ data: [] } as any)
  const { result, rerender } = renderHook(({ server }) => useDirectory(true, server), { initialProps: { server: 1 } })
  await waitFor(() => expect(result.current.ready).toBe(true))
  const same = result.current.capture()
  await act(async () => same.refresh())
  expect(listFiles).toHaveBeenCalledTimes(2)
  const oldPath = result.current.capture()
  await act(async () => { await result.current.loadFiles('/next') })
  act(() => oldPath.refresh())
  expect(listFiles).toHaveBeenCalledTimes(3)
  const oldServer = result.current.capture()
  rerender({ server: 2 })
  act(() => oldServer.refresh())
  expect(listFiles).toHaveBeenCalledTimes(4)
  expect(oldServer.isCurrent()).toBe(false)
})

it('clears stale rows before a new request and never re-enables failed selections', async () => {
  listFiles.mockResolvedValueOnce({ data: [entry('old')] } as any)
  const pending = deferred()
  listFiles.mockReturnValueOnce(pending.promise)
  const { result } = renderHook(() => useDirectory(true, 1))
  await waitFor(() => expect(result.current.ready).toBe(true))
  act(() => { void result.current.loadFiles('/missing') })
  expect(result.current.files).toEqual([])
  expect(result.current.ready).toBe(false)
  await act(async () => pending.reject(new Error('failure')))
  expect(result.current.ready).toBe(false)
  expect(result.current.loading).toBe(false)
})

it('rejects a stale row confirmation even when the same path was refreshed', async () => {
  listFiles.mockResolvedValue({ data: [entry('old')] } as any)
  const { result } = renderHook(() => useDirectory(true, 1))
  await waitFor(() => expect(result.current.ready).toBe(true))
  const oldRowCapture = result.current.capture
  await act(async () => { await result.current.loadFiles('/') })
  expect(oldRowCapture().isCurrent()).toBe(false)
  expect(result.current.capture().isCurrent()).toBe(true)
})
