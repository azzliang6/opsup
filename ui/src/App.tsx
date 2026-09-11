import { lazy, Suspense } from 'react'
import Spin from 'antd/es/spin'
import { Routes, Route, Navigate } from 'react-router-dom'
import ConfigProvider from 'antd/es/config-provider'
import LoginPage from './pages/LoginPage'
import SetupPage from './pages/SetupPage'
const MainPage = lazy(() => import('./pages/MainPage'))
import { ThemeProvider, useTheme } from './contexts/ThemeContext'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const token = localStorage.getItem('opsup_token')
  return token ? <>{children}</> : <Navigate to="/login" />
}

function ThemedApp() {
  const { themeConfig } = useTheme()
  return (
    <ConfigProvider theme={themeConfig}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/setup" element={<SetupPage />} />
        <Route
          path="/*"
          element={
            <PrivateRoute>
              <Suspense fallback={<Spin fullscreen tip="加载工作区…" />}><MainPage /></Suspense>
            </PrivateRoute>
          }
        />
      </Routes>
    </ConfigProvider>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <ThemedApp />
    </ThemeProvider>
  )
}
