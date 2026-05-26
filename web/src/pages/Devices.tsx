import { useEffect, useState } from 'react'
import { fetchDevices, createDevice, deleteDevice, setDefaultDevice, Device, CreateDeviceRequest } from '../api'
import './Pages.css'

function Devices() {
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)

  // Form state
  const [formName, setFormName] = useState('')
  const [formDeviceKey, setFormDeviceKey] = useState('')
  const [formIsDefault, setFormIsDefault] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')

  useEffect(() => {
    loadDevices()
  }, [])

  async function loadDevices() {
    setLoading(true)
    setError('')
    try {
      const res = await fetchDevices()
      setDevices(res.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载设备列表失败')
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setFormError('')
    try {
      const data: CreateDeviceRequest = {
        name: formName,
        device_key: formDeviceKey,
        is_default: formIsDefault,
      }
      await createDevice(data)
      resetForm()
      setShowForm(false)
      loadDevices()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '添加设备失败')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: string, name: string) {
    if (!confirm(`确定要删除设备「${name}」吗？`)) return
    try {
      await deleteDevice(id)
      loadDevices()
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除设备失败')
    }
  }

  async function handleSetDefault(id: string) {
    try {
      await setDefaultDevice(id)
      loadDevices()
    } catch (err) {
      setError(err instanceof Error ? err.message : '设置默认设备失败')
    }
  }

  function resetForm() {
    setFormName('')
    setFormDeviceKey('')
    setFormIsDefault(false)
    setFormError('')
  }

  function maskDeviceKey(key: string): string {
    if (key.length <= 8) return key
    return key.slice(0, 4) + '****' + key.slice(-4)
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h2 className="page-title">设备管理</h2>
          <p className="page-description">管理推送目标设备，配置 Bark Device Key</p>
        </div>
        <button className="btn btn--primary" onClick={() => setShowForm(!showForm)}>
          {showForm ? '取消' : '+ 添加设备'}
        </button>
      </div>

      {showForm && (
        <div className="form-card">
          <h3 className="form-title">添加新设备</h3>
          {formError && <div className="error-banner">{formError}</div>}
          <form onSubmit={handleCreate} className="form-grid">
            <div className="form-group">
              <label className="form-label" htmlFor="device-name">设备名称 *</label>
              <input
                id="device-name"
                type="text"
                className="text-input"
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                required
                maxLength={100}
                placeholder="如：我的 iPhone、iPad Pro"
              />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="device-key">Device Key *</label>
              <input
                id="device-key"
                type="text"
                className="text-input"
                value={formDeviceKey}
                onChange={(e) => setFormDeviceKey(e.target.value)}
                required
                placeholder="从 Bark App 中复制 Device Key"
              />
              <small className="form-hint">
                打开 Bark App → 点击服务器地址 → 复制最后一段即为 Device Key
              </small>
            </div>
            <div className="form-group">
              <label className="form-label form-label--checkbox">
                <input
                  type="checkbox"
                  checked={formIsDefault}
                  onChange={(e) => setFormIsDefault(e.target.checked)}
                />
                设为默认推送设备
              </label>
              <small className="form-hint">默认设备将接收所有未指定设备的推送</small>
            </div>
            <div className="form-actions">
              <button type="submit" className="btn btn--primary" disabled={submitting}>
                {submitting ? '添加中...' : '添加设备'}
              </button>
              <button type="button" className="btn btn--secondary" onClick={() => { resetForm(); setShowForm(false) }}>
                取消
              </button>
            </div>
          </form>
        </div>
      )}

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="placeholder-content"><p>加载中...</p></div>
      ) : devices.length === 0 ? (
        <div className="placeholder-content">
          <p>暂无设备</p>
          <p className="placeholder-hint">添加一个 Bark 设备来开始接收推送通知</p>
        </div>
      ) : (
        <div className="device-list">
          {devices.map((d) => (
            <div key={d.id} className={`device-card ${d.is_default ? 'device-card--default' : ''}`}>
              <div className="device-header">
                <div className="device-info">
                  <h4 className="device-name">
                    {d.name}
                    {d.is_default && <span className="device-badge">默认</span>}
                  </h4>
                  <span className="device-key">{maskDeviceKey(d.device_key)}</span>
                </div>
              </div>
              <div className="device-meta">
                <span>添加时间: {new Date(d.created_at).toLocaleString('zh-CN')}</span>
              </div>
              <div className="device-actions">
                {!d.is_default && (
                  <button
                    className="btn btn--secondary btn--sm"
                    onClick={() => handleSetDefault(d.id)}
                  >
                    设为默认
                  </button>
                )}
                <button
                  className="btn btn--danger btn--sm"
                  onClick={() => handleDelete(d.id, d.name)}
                >
                  删除
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default Devices
