import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import FileManager from './FileManager'
import { ThemeProvider } from '../contexts/ThemeContext'
import * as filesApi from '../api/files'

vi.mock('../api/files', () => ({ listFiles: vi.fn(), deleteFile: vi.fn(), uploadFile: vi.fn(), createDir: vi.fn(), downloadFile: vi.fn() }))
beforeEach(() => vi.mocked(filesApi.listFiles).mockReset())

it('paginates large directories and exposes folders as keyboard buttons', async () => {
  const entries = Array.from({ length: 125 }, (_, index) => ({ name: `folder-${index}`, path: `/folder-${index}`, isDir: true, mode: '', modTime: '', size: 0 }))
  vi.mocked(filesApi.listFiles).mockResolvedValue({ data: entries } as any)
  render(<ThemeProvider><FileManager open serverId={1} serverName="Test" onClose={() => {}} /></ThemeProvider>)
  expect(await screen.findByText('folder-0')).toBeEnabled()
  expect(screen.queryByText('folder-50')).not.toBeInTheDocument()
  fireEvent.click(screen.getByTitle('2'))
  expect(await screen.findByText('folder-50')).toBeInTheDocument()
  expect(screen.queryByText('folder-0')).not.toBeInTheDocument()
  fireEvent.click(screen.getByText('folder-50'))
  await waitFor(() => expect(filesApi.listFiles).toHaveBeenLastCalledWith(1, '/folder-50', expect.any(AbortSignal)))
  expect(screen.getByLabelText('目录路径')).toHaveValue('/folder-50')
})
