import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import { ThemeProvider, useTheme } from './ThemeContext'

function Controls() {
  const { mode, accent, colors, toggleMode, setAccent } = useTheme()
  return <><output>{mode}:{accent}:{colors.terminalBg}</output><button onClick={toggleMode}>Theme</button><button onClick={() => setAccent('purple')}>Accent</button></>
}

it('preserves the terminal palette and applies saved accent choices to CSS states', () => {
  localStorage.setItem('opsup_theme_mode', 'invalid')
  localStorage.setItem('opsup_theme_accent', 'invalid')
  render(<ThemeProvider><Controls /></ThemeProvider>)
  expect(screen.getByRole('status')).toHaveTextContent('dark:blue:#1a1a2e')
  fireEvent.click(screen.getByText('Accent'))
  expect(document.documentElement.style.getPropertyValue('--color-primary')).toBe('#722ed1')
  expect(document.documentElement.style.getPropertyValue('--accent-border')).toBe('#722ed140')
  fireEvent.click(screen.getByText('Theme'))
  expect(screen.getByRole('status')).toHaveTextContent('light:purple:#ffffff')
  expect(document.documentElement.dataset.theme).toBe('light')
  expect(localStorage.getItem('opsup_theme_mode')).toBe('light')
  expect(localStorage.getItem('opsup_theme_accent')).toBe('purple')
})
