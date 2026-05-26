import { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { getApiKey } from '../api'

interface ApiKeyGuardProps {
  children: ReactNode
}

function ApiKeyGuard({ children }: ApiKeyGuardProps) {
  const apiKey = getApiKey()

  if (!apiKey) {
    return (
      <div className="apikey-guard">
        <div className="apikey-guard-card">
          <div className="apikey-guard-icon">🔑</div>
          <h2 className="apikey-guard-title">欢迎使用 Overseer</h2>
          <p className="apikey-guard-desc">
            请先配置 API Key 以连接服务器。你可以在服务器的配置文件中找到 API Key。
          </p>
          <Link to="/settings" className="btn btn--primary">
            前往设置
          </Link>
        </div>
      </div>
    )
  }

  return <>{children}</>
}

export default ApiKeyGuard
