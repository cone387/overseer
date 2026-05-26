import { useState, useEffect } from 'react'
import {
  getMe,
  changePassword,
  logout,
  ApiError,
  type AuthUser,
} from '../api'
import './Pages.css'

function Settings() {
  const [user, setUser] = useState<AuthUser | null>(null)

  // Change password state
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwdLoading, setPwdLoading] = useState(false)
  const [pwdResult, setPwdResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    loadUser()
  }, [])

  async function loadUser() {
    try {
      const me = await getMe()
      setUser(me)
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
