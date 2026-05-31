import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { request } from '../api'

function DesktopLink() {
  const [searchParams] = useSearchParams()
  const code = searchParams.get('code') || ''
  const [status, setStatus] = useState<'confirm' | 'loading' | 'success' | 'error'>('confirm')
  const [error, setError] = useState('')

  const handleConfirm = async () => {
    if (!code) {
      setError('缺少授权码')
      return
    }
    setStatus('loading')
    try {
      await request('/api/devices/desktop-confirm', {
        method: 'POST',
        body: JSON.stringify({ code }),
      })
      setStatus('success')
    } catch (e: any) {
      setError(e.message || '授权失败')
      setStatus('error')
    }
  }

  return (
    <div style={styles.container}>
      <div style={styles.card}>
        <div style={styles.icon}>🖥️</div>
        <h1 style={styles.title}>桌面客户端授权</h1>

        {status === 'confirm' && (
          <>
            <p style={styles.desc}>
              Overseer Desktop 正在请求连接到此服务器。
            </p>
            {code && (
              <div style={styles.codeBox}>
                <span style={styles.codeLabel}>授权码</span>
                <span style={styles.code}>{code}</span>
              </div>
            )}
            <p style={styles.hint}>
              请确认这是你发起的请求，点击下方按钮完成授权。
            </p>
            <button style={styles.btn} onClick={handleConfirm}>
              确认授权
            </button>
          </>
        )}

        {status === 'loading' && (
          <p style={styles.desc}>正在授权...</p>
        )}

        {status === 'success' && (
          <>
            <div style={styles.successIcon}>✅</div>
            <p style={styles.desc}>授权成功！桌面客户端已连接。</p>
            <p style={styles.hint}>你可以关闭此页面。</p>
          </>
        )}

        {status === 'error' && (
          <>
            <p style={{ ...styles.desc, color: '#e55561' }}>{error}</p>
            <button style={styles.btn} onClick={() => setStatus('confirm')}>
              重试
            </button>
          </>
        )}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '100vh',
    background: '#f8f8fa',
    padding: 24,
  },
  card: {
    background: '#fff',
    borderRadius: 12,
    padding: '40px 32px',
    maxWidth: 400,
    width: '100%',
    textAlign: 'center',
    boxShadow: '0 2px 12px rgba(0,0,0,0.08)',
  },
  icon: { fontSize: 48, marginBottom: 16 },
  title: { fontSize: 20, marginBottom: 8, color: '#1a1a1f' },
  desc: { fontSize: 14, color: '#555', marginBottom: 16 },
  hint: { fontSize: 12, color: '#888', marginBottom: 20 },
  codeBox: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 4,
    padding: 16,
    background: '#f5f5f7',
    borderRadius: 8,
    margin: '16px 0',
  },
  codeLabel: { fontSize: 11, color: '#888', textTransform: 'uppercase', letterSpacing: 0.5 },
  code: { fontSize: 28, fontWeight: 700, fontFamily: 'monospace', letterSpacing: 4, color: '#2563eb' },
  successIcon: { fontSize: 48, marginBottom: 12 },
  btn: {
    width: '100%',
    padding: '12px 16px',
    border: 'none',
    borderRadius: 8,
    background: '#2563eb',
    color: '#fff',
    fontSize: 14,
    fontWeight: 600,
    cursor: 'pointer',
  },
}

export default DesktopLink
