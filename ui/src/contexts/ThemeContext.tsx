import { createContext, useContext, useState, useEffect, useMemo } from 'react'
import type { ThemeConfig } from 'antd'
import { theme } from 'antd'

export type ThemeMode = 'dark' | 'light'
export type AccentKey = 'blue' | 'green' | 'purple' | 'orange' | 'red'

const ACCENT_MAP: Record<AccentKey, string> = {
  blue: '#1677ff',
  green: '#52c41a',
  purple: '#722ed1',
  orange: '#fa8c16',
  red: '#f5222d',
}

export const ACCENT_OPTIONS: { key: AccentKey; color: string; label: string }[] = [
  { key: 'blue', color: '#1677ff', label: '蓝色' },
  { key: 'green', color: '#52c41a', label: '绿色' },
  { key: 'purple', color: '#722ed1', label: '紫色' },
  { key: 'orange', color: '#fa8c16', label: '橙色' },
  { key: 'red', color: '#f5222d', label: '红色' },
]

export interface SemanticColors {
  bgBase: string
  bgSidebar: string
  bgTabBar: string
  bgCardOverlay: string
  textPrimary: string
  textSecondary: string
  textTertiary: string
  borderSubtle: string
  borderLight: string
  fillHover: string
  fillSubtle: string
  fillMedium: string
  scrollbarThumb: string
  scrollbarHover: string
  bgContextMenu: string
  shadowContextMenu: string
  gradientPage: string
  gradientLogo: string
  colorPrimary: string
  terminalBg: string
  terminalFg: string
}

function getSemanticColors(mode: ThemeMode, accent: AccentKey): SemanticColors {
  const primary = ACCENT_MAP[accent]
  if (mode === 'dark') {
    return {
      bgBase: '#141414',
      bgSidebar: '#1a1a2e',
      bgTabBar: '#1a1a2e',
      bgCardOverlay: 'rgba(30, 30, 50, 0.9)',
      textPrimary: '#e0e0e0',
      textSecondary: 'rgba(255,255,255,0.65)',
      textTertiary: 'rgba(255,255,255,0.35)',
      borderSubtle: 'rgba(255,255,255,0.06)',
      borderLight: 'rgba(255,255,255,0.04)',
      fillHover: 'rgba(255,255,255,0.07)',
      fillSubtle: 'rgba(255,255,255,0.03)',
      fillMedium: 'rgba(255,255,255,0.06)',
      scrollbarThumb: 'rgba(255, 255, 255, 0.15)',
      scrollbarHover: 'rgba(255, 255, 255, 0.3)',
      bgContextMenu: '#2a2a2a',
      shadowContextMenu: '0 6px 16px rgba(0,0,0,0.5)',
      gradientPage: 'linear-gradient(135deg, #0c0c1d 0%, #1a1a2e 50%, #16213e 100%)',
      gradientLogo: `linear-gradient(135deg, ${primary}, ${primary}cc)`,
      colorPrimary: primary,
      terminalBg: '#1a1a2e',
      terminalFg: '#e0e0e0',
    }
  }
  return {
    bgBase: '#f0f2f5',
    bgSidebar: '#ffffff',
    bgTabBar: '#ffffff',
    bgCardOverlay: 'rgba(255, 255, 255, 0.95)',
    textPrimary: '#141414',
    textSecondary: 'rgba(0,0,0,0.65)',
    textTertiary: 'rgba(0,0,0,0.35)',
    borderSubtle: 'rgba(0,0,0,0.06)',
    borderLight: 'rgba(0,0,0,0.04)',
    fillHover: 'rgba(0,0,0,0.04)',
    fillSubtle: 'rgba(0,0,0,0.02)',
    fillMedium: 'rgba(0,0,0,0.06)',
    scrollbarThumb: 'rgba(0, 0, 0, 0.15)',
    scrollbarHover: 'rgba(0, 0, 0, 0.3)',
    bgContextMenu: '#ffffff',
    shadowContextMenu: '0 6px 16px rgba(0,0,0,0.12)',
    gradientPage: 'linear-gradient(135deg, #e8eaf6 0%, #c5cae9 50%, #bbdefb 100%)',
    gradientLogo: `linear-gradient(135deg, ${primary}, ${primary}cc)`,
    colorPrimary: primary,
    terminalBg: '#ffffff',
    terminalFg: '#1a1a1a',
  }
}

const MODE_TOKENS: Record<ThemeMode, Record<string, string>> = {
  dark: {
    colorBgContainer: '#1f1f1f',
    colorBgElevated: '#2a2a2a',
    colorBorder: '#3a3a3a',
  },
  light: {
    colorBgContainer: '#ffffff',
    colorBgElevated: '#f5f5f5',
    colorBorder: '#e8e8e8',
  },
}

interface ThemeContextValue {
  mode: ThemeMode
  accent: AccentKey
  setMode: (mode: ThemeMode) => void
  setAccent: (accent: AccentKey) => void
  toggleMode: () => void
  themeConfig: ThemeConfig
  colors: SemanticColors
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}

function loadMode(): ThemeMode {
  return (localStorage.getItem('opsup_theme_mode') as ThemeMode) || 'dark'
}

function loadAccent(): AccentKey {
  return (localStorage.getItem('opsup_theme_accent') as AccentKey) || 'blue'
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(loadMode)
  const [accent, setAccentState] = useState<AccentKey>(loadAccent)

  const setMode = (m: ThemeMode) => {
    setModeState(m)
    localStorage.setItem('opsup_theme_mode', m)
  }

  const setAccent = (a: AccentKey) => {
    setAccentState(a)
    localStorage.setItem('opsup_theme_accent', a)
  }

  const toggleMode = () => setMode(mode === 'dark' ? 'light' : 'dark')

  const colors = useMemo(() => getSemanticColors(mode, accent), [mode, accent])

  const themeConfig = useMemo<ThemeConfig>(() => ({
    algorithm: mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary: ACCENT_MAP[accent],
      borderRadius: 6,
      ...MODE_TOKENS[mode],
    },
  }), [mode, accent])

  // Inject CSS custom properties
  useEffect(() => {
    const root = document.documentElement
    root.style.setProperty('--bg-base', colors.bgBase)
    root.style.setProperty('--bg-sidebar', colors.bgSidebar)
    root.style.setProperty('--bg-terminal', colors.terminalBg)
    root.style.setProperty('--text-primary', colors.textPrimary)
    root.style.setProperty('--text-secondary', colors.textSecondary)
    root.style.setProperty('--text-tertiary', colors.textTertiary)
    root.style.setProperty('--scrollbar-thumb', colors.scrollbarThumb)
    root.style.setProperty('--scrollbar-hover', colors.scrollbarHover)
    root.style.setProperty('--fill-hover', colors.fillHover)
    root.style.setProperty('--fill-subtle', colors.fillSubtle)
    root.style.setProperty('--border-subtle', colors.borderSubtle)
  }, [colors])

  return (
    <ThemeContext.Provider value={{ mode, accent, setMode, setAccent, toggleMode, themeConfig, colors }}>
      {children}
    </ThemeContext.Provider>
  )
}
