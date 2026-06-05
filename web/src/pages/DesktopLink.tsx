import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { confirmDesktopLink } from '../api'
import './Pages.css'

export default function DesktopLink() {
  const [searchParams] = useSearchParams()
  const code = searchParams.get('code') || ''
  const [status, setStatus] = useState<'idle' | 'confirming' | 'success' | 'error'>('idle')
  const [error, setError] = useState('')

  const handleConfirm = async () => {
    if (!code) {
      setError('缺少授权码')
      return
    }
    setStatus('confirming')
    setError('')
    try {
      await confirmDesktopLink(code)
      setStatus('success')
    } catch (e: any) {
      setError(e.message || '确认失败')
      setStatus('error')
    }
  }

  if (!code) {
    return (
      <div className="desktop-link-page">
        <div className="desktop-link-card">
          <div className="desktop-link-icon">⚠️</div>
          <h2>无效的授权链接</h2>
          <p>缺少授权码参数</p>
        </div>
      </div>
    )
  }

  if (status === 'success') {
    return (
      <div className="desktop-link-page">
        <div className="desktop-link-card">
          <div className="desktop-link-icon">✅</div>
          <h2>授权成功</h2>
          <p>桌面客户端已连接，可以关闭此页面</p>
        </div>
      </div>
    )
  }

  return (
    <div className="desktop-link-page">
      <div className="desktop-link-card">
        <div className="desktop-link-icon">🖥️</div>
        <h2>桌面客户端授权</h2>
        <p>确认将以下设备连接到 Overseer？</p>
        <div className="desktop-link-code">{code}</div>
        {error && <p className="desktop-link-error">{error}</p>}
        <button
          className="btn--primary desktop-link-btn"
          onClick={handleConfirm}
          disabled={status === 'confirming'}
        >
          {status === 'confirming' ? '确认中...' : '确认授权'}
        </button>
      </div>
    </div>
  )
}
