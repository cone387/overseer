import { useState, useEffect } from 'react'
import {
  getMe,
  changePassword,
  logout,
  getLLMSettings,
  saveLLMSettings,
  fetchLLMModels,
  ApiError,
  type AuthUser,
  type LLMSettings,
} from '../api'
import './Pages.css'

function Settings() {
  const [user, setUser] = useState<AuthUser | null>(null)

  // LLM settings state
  const [llmSettings, setLlmSettings] = useState<LLMSettings | null>(null)
  const [llmBaseUrl, setLlmBaseUrl] = useState('')
  const [llmApiKey, setLlmApiKey] = useState('')
  const [llmModel, setLlmModel] = useState('')
  const [llmModels, setLlmModels] = useState<string[]>([])
  const [llmModelSearch, setLlmModelSearch] = useState('')
  const [llmModelsLoading, setLlmModelsLoading] = useState(false)
  const [llmSaving, setLlmSaving] = useState(false)
  const [llmResult, setLlmResult] = useState<{ ok: boolean; message: string } | null>(null)

  // Change password state
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwdLoading, setPwdLoading] = useState(false)
  const [pwdResult, setPwdResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    loadUser()
    loadLLMSettings()
  }, [])

  async function loadUser() {
    try {
      const me = await getMe()
      setUser(me)
    } catch {
      // ignore
    }
  }

  async function loadLLMSettings() {
    try {
      const res = await getLLMSettings()
      setLlmSettings(res.data)
      setLlmBaseUrl(res.data.base_url || '')
      setLlmModel(res.data.model || '')
      setLlmModelSearch(res.data.model || '')
      // Auto-load models if configured
      if (res.data.configured) {
        loadModels()
      }
    } catch { /* ignore */ }
  }

  async function loadModels() {
    setLlmModelsLoading(true)
    try {
      const res = await fetchLLMModels()
      setLlmModels(res.data || [])
    } catch { /* ignore */ }
    finally { setLlmModelsLoading(false) }
  }

  async function handleSaveLLM() {
    setLlmSaving(true)
    setLlmResult(null)
    try {
      await saveLLMSettings({
        base_url: llmBaseUrl,
        api_key: llmApiKey || undefined,
        model: llmModel,
      })
      setLlmResult({ ok: true, message: 'LLM 配置已保存' })
      setLlmApiKey('')
      loadLLMSettings()
    } catch (err) {
      setLlmResult({ ok: false, message: err instanceof ApiError ? err.message : '保存失败' })
    } finally {
      setLlmSaving(false)
    }
  }

  async function handleChangePassword() {
    setPwdResult(null)
    if (newPassword !== confirmPassword) {
      setPwdResult({ ok: false, message: '两次输入的密码不一致' })
      return
    }
    if (newPassword.length < 6) {
      setPwdResult({ ok: false, message: '新密码长度至少 6 位' })
      return
    }
    setPwdLoading(true)
    try {
      await changePassword(oldPassword, newPassword)
      setPwdResult({ ok: true, message: '密码修改成功' })
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      setPwdResult({ ok: false, message: err instanceof ApiError ? err.message : '修改失败' })
    } finally {
      setPwdLoading(false)
    }
  }

  async function handleLogout() {
    await logout()
    window.location.reload()
  }

  return (
    <div className="page">
      <h2 className="page-title">设置</h2>
      <p className="page-description">账户管理</p>

      {/* User Info */}
      {user && (
        <div className="form-card">
          <h3 className="form-title">当前用户</h3>
          <div className="form-grid">
            <div className="form-group">
              <label className="form-label">用户名</label>
              <div className="settings-current-key">{user.username}</div>
            </div>
          </div>
        </div>
      )}

      {/* Change Password */}
      <div className="form-card">
        <h3 className="form-title">修改密码</h3>
        <div className="form-grid">
          <div className="form-group">
            <label className="form-label" htmlFor="old-pwd">当前密码</label>
            <input
              id="old-pwd"
              type="password"
              className="text-input"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              placeholder="输入当前密码"
              autoComplete="current-password"
            />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="new-pwd">新密码</label>
              <input
                id="new-pwd"
                type="password"
                className="text-input"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="至少 6 位"
                autoComplete="new-password"
              />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="confirm-pwd">确认新密码</label>
              <input
                id="confirm-pwd"
                type="password"
                className="text-input"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="再次输入新密码"
                autoComplete="new-password"
              />
            </div>
          </div>
          <div className="form-actions">
            <button
              className="btn btn--primary"
              onClick={handleChangePassword}
              disabled={pwdLoading || !oldPassword || !newPassword}
            >
              {pwdLoading ? '修改中...' : '修改密码'}
            </button>
          </div>
        </div>
        {pwdResult && (
          <div className={`result-card ${pwdResult.ok ? 'result-card--success' : 'result-card--error'}`}>
            <div className="result-icon">{pwdResult.ok ? '✓' : '✗'}</div>
            <div className="result-content">
              <div className="result-message">{pwdResult.message}</div>
            </div>
          </div>
        )}
      </div>

      {/* LLM Configuration */}
      <div className="form-card">
        <h3 className="form-title">AI 智能解析配置</h3>
        <p className="form-hint" style={{ marginBottom: '1rem' }}>配置 OpenAI 兼容的 LLM API，用于自然语言解析定时提醒。支持 OpenAI、DeepSeek、通义千问等。</p>
        <div className="form-grid">
          <div className="form-group">
            <label className="form-label" htmlFor="llm-base-url">API 地址</label>
            <input id="llm-base-url" type="url" className="text-input" value={llmBaseUrl} onChange={(e) => setLlmBaseUrl(e.target.value)} placeholder="https://api.openai.com/v1" />
            <small className="form-hint">留空默认使用 OpenAI 官方地址</small>
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="llm-api-key">API Key {llmSettings?.configured && <span style={{ color: '#059669' }}>（已配置）</span>}</label>
            <input id="llm-api-key" type="password" className="text-input" value={llmApiKey} onChange={(e) => setLlmApiKey(e.target.value)} placeholder={llmSettings?.configured ? '留空保持不变，输入新值则覆盖' : '输入 API Key'} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="llm-model">模型</label>
            <div style={{ position: 'relative' }}>
              <input
                id="llm-model"
                type="text"
                className="text-input"
                value={llmModelSearch}
                onChange={(e) => { setLlmModelSearch(e.target.value); setLlmModel(e.target.value) }}
                placeholder={llmModelsLoading ? '加载模型列表中...' : 'gpt-4o-mini'}
                list="llm-model-list"
              />
              <datalist id="llm-model-list">
                {llmModels
                  .filter((m) => !llmModelSearch || m.toLowerCase().includes(llmModelSearch.toLowerCase()))
                  .slice(0, 50)
                  .map((m) => (<option key={m} value={m} />))
                }
              </datalist>
            </div>
            <small className="form-hint">
              {llmModels.length > 0
                ? `已加载 ${llmModels.length} 个可用模型，输入搜索`
                : '保存 API 配置后自动获取可用模型列表'}
              {llmSettings?.configured && !llmModelsLoading && llmModels.length === 0 && (
                <button type="button" className="btn btn--link" style={{ marginLeft: '0.5rem' }} onClick={loadModels}>刷新模型列表</button>
              )}
            </small>
          </div>
          <div className="form-actions">
            <button className="btn btn--primary" onClick={handleSaveLLM} disabled={llmSaving}>
              {llmSaving ? '保存中...' : '保存配置'}
            </button>
          </div>
        </div>
        {llmResult && (
          <div className={`result-card ${llmResult.ok ? 'result-card--success' : 'result-card--error'}`} style={{ marginTop: '0.75rem' }}>
            <div className="result-icon">{llmResult.ok ? '✓' : '✗'}</div>
            <div className="result-content"><div className="result-message">{llmResult.message}</div></div>
          </div>
        )}
      </div>

      {/* Logout */}
      <div className="form-card">
        <div className="form-actions">
          <button className="btn btn--danger" onClick={handleLogout}>
            退出登录
          </button>
        </div>
      </div>
    </div>
  )
}

export default Settings
