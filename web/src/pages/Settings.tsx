import { useState } from 'react'
import { getApiKey, setApiKey } from '../api'
import './Pages.css'

function Settings() {
  const [key, setKey] = useState(getApiKey())
  const [saved, setSaved] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  function handleSave() {
    setApiKey(key.trim())
    setSaved(true)
    setTestResult(null)
    setTimeout(() => setSaved(false), 2000)
  }

  async function handleTest() {
    setTesting(true)
    setTestResult(null)
    try {
      const res = await fetch('/health', {
        headers: key.trim() ? { 'X-API-Key': key.trim() } : {},
      })
      if (res.ok) {
        setTestResult({ ok: true, message: '连接成功！服务器正常运行。' })
      } else {
        const json = await res.json().catch(() => null)
        setTestResult({ ok: false, message: json?.message || `连接失败 (HTTP ${res.status})` })
      }
    } catch (err) {
      setTestResult({ ok: false, message: err instanceof Error ? err.message : '网络错误，无法连接服务器' })
    } finally {
      setTesting(false)
    }
  }

  function maskKey(k: string): string {
    if (k.length <= 8) return '••••••••'
    return k.slice(0, 4) + '••••' + k.slice(-4)
  }

  return (
    <div className="page">
      <h2 className="page-title">设置</h2>
      <p className="page-description">配置 API Key 以连接 Overseer 服务器</p>

      <div className="form-card">
        <div className="form-grid">
          {getApiKey() && (
            <div className="form-group">
              <label className="form-label">当前 API Key</label>
              <div className="settings-current-key">{maskKey(getApiKey())}</div>
            </div>
          )}

          <div className="form-group">
            <label className="form-label" htmlFor="settings-apikey">API Key</label>
            <input
              id="settings-apikey"
              type="password"
              className="text-input"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="输入你的 API Key"
            />
            <small className="form-hint">API Key 用于验证身份，存储在浏览器本地</small>
          </div>

          <div className="form-actions">
            <button
              className="btn btn--primary"
              onClick={handleSave}
              disabled={!key.trim()}
            >
              {saved ? '✓ 已保存' : '保存'}
            </button>
            <button
              className="btn btn--secondary"
              onClick={handleTest}
              disabled={testing}
            >
              {testing ? '测试中...' : '测试连接'}
            </button>
          </div>
        </div>
      </div>

      {testResult && (
        <div className={`result-card ${testResult.ok ? 'result-card--success' : 'result-card--error'}`}>
          <div className="result-icon">{testResult.ok ? '✓' : '✗'}</div>
          <div className="result-content">
            <div className="result-message">{testResult.message}</div>
          </div>
        </div>
      )}
    </div>
  )
}

export default Settings
