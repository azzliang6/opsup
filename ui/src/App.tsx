import { Routes, Route, Navigate } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import LoginPage from './pages/LoginPage'
import SetupPage from './pages/SetupPage'
import MainPage from './pages/MainPage'
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
              <MainPage />
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
