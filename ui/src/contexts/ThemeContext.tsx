import { createContext, useContext, useState, useEffect, useMemo } from 'react'
import type { ThemeConfig } from 'antd'
import theme from 'antd/es/theme'

export type ThemeMode = 'dark' | 'light'
export type AccentKey = 'blue' | 'green' | 'purple' | 'orange' | 'red'

const ACCENT_MAP: Record<AccentKey, string> = {
  blue: '#1677ff', green: '#52c41a', purple: '#722ed1', orange: '#fa8c16', red: '#f5222d',
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
  const shared = { colorPrimary: primary, gradientLogo: `linear-gradient(145deg, ${primary}, ${primary}bb)` }
  return mode === 'dark' ? {
    ...shared,
    bgBase: '#131320', bgSidebar: '#181827', bgTabBar: '#161625',
    bgCardOverlay: 'rgba(26, 26, 46, 0.96)',
    textPrimary: '#eeeef6', textSecondary: '#b4b4c9', textTertiary: '#9090aa',
    borderSubtle: 'rgba(180,180,220,0.12)', borderLight: 'rgba(180,180,220,0.07)',
    fillHover: 'rgba(180,180,220,0.09)', fillSubtle: 'rgba(180,180,220,0.035)', fillMedium: 'rgba(180,180,220,0.07)',
    scrollbarThumb: 'rgba(180,180,220,0.2)', scrollbarHover: 'rgba(180,180,220,0.35)',
    bgContextMenu: '#222238', shadowContextMenu: '0 12px 36px rgba(0,0,0,0.3)',
    gradientPage: 'radial-gradient(ellipse at 50% 20%, #242444 0%, #171728 48%, #10101d 100%)',
    terminalBg: '#1a1a2e', terminalFg: '#e0e0e0',
  } : {
    ...shared,
    bgBase: '#f3f5f9', bgSidebar: '#fcfcfe', bgTabBar: '#f8f9fc',
    bgCardOverlay: 'rgba(255,255,255,0.97)',
    textPrimary: '#23243b', textSecondary: '#5d617a', textTertiary: '#70768c',
    borderSubtle: 'rgba(56,65,99,0.12)', borderLight: 'rgba(56,65,99,0.07)',
    fillHover: 'rgba(56,65,99,0.06)', fillSubtle: 'rgba(56,65,99,0.025)', fillMedium: 'rgba(56,65,99,0.05)',
    scrollbarThumb: 'rgba(56,65,99,0.2)', scrollbarHover: 'rgba(56,65,99,0.35)',
    bgContextMenu: '#ffffff', shadowContextMenu: '0 12px 36px rgba(36,44,80,0.12)',
    gradientPage: 'radial-gradient(ellipse at 50% 20%, #e4e8fa 0%, #eef0f8 48%, #f6f7fb 100%)',
    terminalBg: '#ffffff', terminalFg: '#1a1a1a',
  }
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
  return localStorage.getItem('opsup_theme_mode') === 'light' ? 'light' : 'dark'
}
function loadAccent(): AccentKey {
  const stored = localStorage.getItem('opsup_theme_accent')
  return ACCENT_OPTIONS.find(option => option.key === stored)?.key || 'blue'
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(loadMode)
  const [accent, setAccentState] = useState<AccentKey>(loadAccent)
  const setMode = (value: ThemeMode) => {
    setModeState(value)
    localStorage.setItem('opsup_theme_mode', value)
  }
  const setAccent = (value: AccentKey) => {
    setAccentState(value)
    localStorage.setItem('opsup_theme_accent', value)
  }
  const toggleMode = () => setMode(mode === 'dark' ? 'light' : 'dark')
  const colors = useMemo(() => getSemanticColors(mode, accent), [mode, accent])
  const themeConfig = useMemo<ThemeConfig>(() => ({
    algorithm: mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary: colors.colorPrimary,
      colorBgContainer: mode === 'dark' ? '#1e1e32' : '#ffffff',
      colorBgElevated: colors.bgContextMenu,
      colorBgLayout: colors.bgBase,
      colorBorder: mode === 'dark' ? '#38384f' : '#dfe3ed',
      colorText: colors.textPrimary,
      colorTextSecondary: colors.textSecondary,
      colorTextPlaceholder: colors.textTertiary,
      fontFamily: 'var(--font-ui)',
      borderRadius: 8,
      controlHeight: 34,
    },
    components: {
      Modal: { borderRadiusLG: 12 },
      Table: { headerBg: colors.fillSubtle, rowHoverBg: colors.fillHover },
      Button: { fontWeight: 500 },
    },
  }), [mode, colors])

  useEffect(() => {
    const root = document.documentElement
    root.dataset.theme = mode
    const variables: Record<string, string> = {
      'bg-base': colors.bgBase, 'bg-sidebar': colors.bgSidebar, 'bg-tab-bar': colors.bgTabBar,
      'bg-terminal': colors.terminalBg, 'bg-elevated': colors.bgContextMenu,
      'text-primary': colors.textPrimary, 'text-secondary': colors.textSecondary, 'text-tertiary': colors.textTertiary,
      'scrollbar-thumb': colors.scrollbarThumb, 'scrollbar-hover': colors.scrollbarHover,
      'fill-hover': colors.fillHover, 'fill-subtle': colors.fillSubtle, 'fill-medium': colors.fillMedium,
      'border-subtle': colors.borderSubtle, 'border-light': colors.borderLight,
      'color-primary': colors.colorPrimary, 'accent-soft': `${colors.colorPrimary}14`, 'accent-border': `${colors.colorPrimary}40`,
      'gradient-page': colors.gradientPage, 'gradient-logo': colors.gradientLogo, 'bg-card-overlay': colors.bgCardOverlay,
    }
    for (const [name, value] of Object.entries(variables)) root.style.setProperty(`--${name}`, value)
  }, [mode, colors])

  return <ThemeContext.Provider value={{ mode, accent, setMode, setAccent, toggleMode, themeConfig, colors }}>{children}</ThemeContext.Provider>
}
