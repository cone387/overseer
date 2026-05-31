import { useState, useEffect, ReactNode, FormEvent } from 'react'
import { checkAuthStatus, login, register, ApiError } from '../api'
import './AuthGuard.css'

interface AuthGuardProps {
  children: ReactNode
}

type AuthState = 'loading' | 'register' | 'login' | 'authenticated'

function AuthGuard({ children }: AuthGuardProps) {
  const [state, setState] = useState<AuthState>('loading')
  const [error, setError] = useState('')

  useEffect(() => {
    checkAuth()

    // Listen for token expiry events from API layer
    const handleAuthExpired = () => {
      setState('login')
      setError('登录已过期，请重新登录')
    }
    window.addEventListener('auth-expired', handleAuthExpired)
    return () => window.removeEventListener('auth-expired', handleAuthExpired)
  }, [])

  async function checkAuth() {
    setState('loading')
    try {
      const status = await checkAuthStatus()
      if (!status.initialized) {
        setState('register')
      } else if (!status.authenticated) {
        setState('login')
      } else {
        setState('authenticated')
      }
    } catch {
      setState('login')
    }
  }

  if (state === 'loading') {
    return (
      <div className="auth-loading">
        <div className="auth-spinner" />
        <p>正在检查认证状态...</p>
      </div>
    )
  }

  if (state === 'register') {
    return (
      <RegisterForm
        error={error}
        onError={setError}
        onSuccess={() => { setError(''); checkAuth() }}
      />
    )
  }

  if (state === 'login') {
    return (
      <LoginForm
        error={error}
        onError={setError}
        onSuccess={() => { setError(''); checkAuth() }}
      />
    )
  }

  return <>{children}</>
}

// ─── Register Form ───────────────────────────────────────────────────────────

interface FormProps {
  error: string
  onError: (msg: string) => void
  onSuccess: () => void
}

function RegisterForm({ error, onError, onSuccess }: FormProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    onError('')

    if (password !== confirmPassword) {
      onError('两次输入的密码不一致')
      return
    }
    if (password.length < 6) {
      onError('密码长度至少 6 位')
      return
    }

    setLoading(true)
    try {
      await register(username.trim(), password)
      onSuccess()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : '注册失败，请重试')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-container">
      <div className="auth-card">
        <div className="auth-icon">🚀</div>
        <h2 className="auth-title">初始化 Overseer</h2>
        <p className="auth-desc">
          这是首次使用，请创建管理员账户。此账户将用于管理推送系统。
        </p>
        {error && <div className="auth-error">{error}</div>}
        <form className="auth-form" onSubmit={handleSubmit}>
          <div className="auth-field">
            <label htmlFor="reg-username">用户名</label>
            <input
              id="reg-username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="输入用户名"
              required
              autoFocus
              autoComplete="username"
            />
          </div>
          <div className="auth-field">
            <label htmlFor="reg-password">密码</label>
            <input
              id="reg-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="至少 6 位"
              required
              autoComplete="new-password"
            />
          </div>
          <div className="auth-field">
            <label htmlFor="reg-confirm">确认密码</label>
            <input
              id="reg-confirm"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="再次输入密码"
              required
              autoComplete="new-password"
            />
          </div>
          <button
            type="submit"
            className="auth-submit"
            disabled={loading || !username.trim() || !password}
          >
            {loading ? '创建中...' : '创建账户'}
          </button>
        </form>
      </div>
    </div>
  )
}

// ─── Login Form ──────────────────────────────────────────────────────────────

function LoginForm({ error, onError, onSuccess }: FormProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    onError('')
    setLoading(true)
    try {
      await login(username.trim(), password)
      onSuccess()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : '登录失败，请重试')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-container">
      <div className="auth-card">
        <div className="auth-icon">🔐</div>
        <h2 className="auth-title">登录 Overseer</h2>
        <p className="auth-desc">请输入账户信息以继续</p>
        {error && <div className="auth-error">{error}</div>}
        <form className="auth-form" onSubmit={handleSubmit}>
          <div className="auth-field">
            <label htmlFor="login-username">用户名</label>
            <input
              id="login-username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="输入用户名"
              required
              autoFocus
              autoComplete="username"
            />
          </div>
          <div className="auth-field">
            <label htmlFor="login-password">密码</label>
            <input
              id="login-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入密码"
              required
              autoComplete="current-password"
            />
          </div>
          <button
            type="submit"
            className="auth-submit"
            disabled={loading || !username.trim() || !password}
          >
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
      </div>
    </div>
  )
}

export default AuthGuard
