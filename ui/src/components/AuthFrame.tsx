import type { ReactNode } from 'react'
import Card from 'antd/es/card'

interface Props {
  title: string
  description: string
  children: ReactNode
}

export default function AuthFrame({ title, description, children }: Props) {
  return (
    <main className="auth-page">
      <Card className="auth-card">
        <header className="auth-heading">
          <span className="brand-mark" aria-hidden="true">&gt;_</span>
          <h1>{title}</h1>
          <p>{description}</p>
        </header>
        {children}
        <footer className="auth-note">OpsUp <span aria-hidden="true">·</span> Web SSH Terminal</footer>
      </Card>
    </main>
  )
}
