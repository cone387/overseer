import { useState, useEffect } from 'react'
import {
  getMe,
  changePassword,
  logout,
  listAPIKeys,
  createAPIKey,
  deleteAPIKey,
  ApiError,
  type AuthUser,
  type APIKey,
} from '../api'
import './Pages.css'

function Settings() {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [keys, setKeys] = useState<APIKey[]>([])
  const [newKeyName, setNewKeyName] = useState('')
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<APIKey | null>(null)
  const [copied, setCopied] = useState(false)

  // Change password state
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwdLoading, setPwdLoading] = useState(false)
  const [pwdResult, setPwdResult] = useState<{ ok: boolean; message: string } | null>(null)

  const [error, setError] = useState('')

  useEffect(() => {
    loadUser()
    loadKeys()
  }, [])

  async function loadUser() {
    try {
      const me = await getMe()
      setUser(me)
    } catch {
      // ignore
    }
  }

  async function loadKeys() {
    try {
      const list = await listAPIKeys()
      setKeys(list)
    } catch {
      // ignore
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

  async function handleCreateKey() {
    if (!newKeyName.trim()) return
    setError('')
    try {
      const key = await createAPIKey(newKeyName.trim())
      setNewlyCreatedKey(key)
      setNewKeyName('')
      loadKeys()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '创建失败')
    }
  }

  async function handleDeleteKey(id: string) {
    try {
      await deleteAPIKey(id)
      setKeys((prev) => prev.filter((k) => k.id !== id))
      if (newlyCreatedKey?.id === id) {
        setNewlyCreatedKey(null)
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '删除失败')
    }
  }

  async function handleCopyKey(key: string) {
    try {
      await navigator.clipboard.writeText(key)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // fallback
    }
  }

  async function handleLogout() {
    await logout()
    window.location.reload()
  }

  return (
    <div className="page">
      <h2 className="page-title">设置</h2>
      <p className="page-description">账户管理与 API Key 配置</p>

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

      {/* API Keys */}
      <div className="form-card">
        <h3 className="form-title">API Key 管理</h3>
        <p className="form-hint" style={{ marginBottom: 16 }}>
          API Key 用于外部程序调用推送接口。生成后完整密钥仅显示一次，请妥善保存。
        </p>

        {error && <div className="error-banner">{error}</div>}

        {/* New key creation */}
        <div className="form-grid" style={{ marginBottom: 16 }}>
          <div className="form-row">
            <div className="form-group" style={{ flex: 1 }}>
              <input
                type="text"
                className="text-input"
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                placeholder="Key 名称（如：服务器A）"
                onKeyDown={(e) => { if (e.key === 'Enter') handleCreateKey() }}
              />
            </div>
            <button
              className="btn btn--primary"
              onClick={handleCreateKey}
              disabled={!newKeyName.trim()}
            >
              生成新 Key
            </button>
          </div>
        </div>

        {/* Newly created key display */}
        {newlyCreatedKey?.key && (
          <div className="result-card result-card--success" style={{ marginBottom: 16 }}>
            <div className="result-content" style={{ flex: 1 }}>
              <div className="result-message">新 Key 已生成：{newlyCreatedKey.name}</div>
              <div className="settings-new-key-display">
                <code className="settings-key-value">{newlyCreatedKey.key}</code>
                <button
                  className="btn btn--secondary btn--sm"
                  onClick={() => handleCopyKey(newlyCreatedKey.key!)}
                >
                  {copied ? '✓ 已复制' : '复制'}
                </button>
              </div>
              <small className="form-hint">⚠️ 此密钥仅显示一次，关闭后无法再次查看</small>
            </div>
          </div>
        )}

        {/* Key list */}
        {keys.length > 0 ? (
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>前缀</th>
                  <th>创建时间</th>
                  <th>最后使用</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => (
                  <tr key={k.id}>
                    <td>{k.name}</td>
                    <td><code>{k.prefix}••••</code></td>
                    <td className="td-nowrap">{formatDate(k.created_at)}</td>
                    <td className="td-nowrap">{k.last_used ? formatDate(k.last_used) : '从未使用'}</td>
                    <td>
                      <button
                        className="btn btn--danger btn--sm"
                        onClick={() => handleDeleteKey(k.id)}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="table-empty">暂无 API Key</div>
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

function formatDate(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return iso
  }
}

export default Settings
