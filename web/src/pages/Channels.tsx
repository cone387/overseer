import { useState, useEffect } from 'react'
import { fetchChannels, createChannel, updateChannel, deleteChannel, fetchDevices, Channel, Device } from '../api'
import './Pages.css'

const LEVEL_OPTIONS = [
  { value: '', label: '默认' },
  { value: 'active', label: 'Active - 普通通知' },
  { value: 'timeSensitive', label: 'Time Sensitive - 时效性通知' },
  { value: 'passive', label: 'Passive - 静默通知' },
  { value: 'critical', label: 'Critical - 紧急通知' },
]

const SOUND_PRESETS = [
  { value: '', label: '默认声音' },
  { value: 'alarm.caf', label: '🚨 alarm - 警报' },
  { value: 'anticipate.caf', label: '⏳ anticipate - 期待' },
  { value: 'bell.caf', label: '🔔 bell - 铃声' },
  { value: 'birdsong.caf', label: '🐦 birdsong - 鸟鸣' },
  { value: 'bloom.caf', label: '🌸 bloom - 绽放' },
  { value: 'calypso.caf', label: '🎵 calypso - 卡利普索' },
  { value: 'chime.caf', label: '🎐 chime - 风铃' },
  { value: 'choo.caf', label: '🚂 choo - 火车' },
  { value: 'descent.caf', label: '⬇️ descent - 下降' },
  { value: 'electronic.caf', label: '⚡ electronic - 电子' },
  { value: 'fanfare.caf', label: '🎺 fanfare - 号角' },
  { value: 'glass.caf', label: '🥂 glass - 玻璃' },
  { value: 'gotosleep.caf', label: '😴 gotosleep - 入睡' },
  { value: 'healthnotification.caf', label: '❤️ health - 健康' },
  { value: 'horn.caf', label: '📯 horn - 号角' },
  { value: 'ladder.caf', label: '🪜 ladder - 阶梯' },
  { value: 'mailsent.caf', label: '📧 mailsent - 邮件发送' },
  { value: 'minuet.caf', label: '🎼 minuet - 小步舞曲' },
  { value: 'multiwayinvitation.caf', label: '📞 multiwayinvitation - 多方邀请' },
  { value: 'newmail.caf', label: '📬 newmail - 新邮件' },
  { value: 'newsflash.caf', label: '📰 newsflash - 新闻快讯' },
  { value: 'noir.caf', label: '🎬 noir - 黑色电影' },
  { value: 'paymentsuccess.caf', label: '💰 paymentsuccess - 支付成功' },
  { value: 'shake.caf', label: '📳 shake - 震动' },
  { value: 'sherwoodforest.caf', label: '🌲 sherwoodforest - 森林' },
  { value: 'silence.caf', label: '🔇 silence - 静音' },
  { value: 'spell.caf', label: '✨ spell - 魔法' },
  { value: 'suspense.caf', label: '😱 suspense - 悬疑' },
  { value: 'telegraph.caf', label: '📠 telegraph - 电报' },
  { value: 'tiptoes.caf', label: '🤫 tiptoes - 蹑手蹑脚' },
  { value: 'typewriters.caf', label: '⌨️ typewriters - 打字机' },
  { value: 'update.caf', label: '🔄 update - 更新' },
]

interface ChannelForm {
  name: string
  sound: string
  group: string
  icon: string
  level: string
  device_keys: string[]
}

const emptyForm: ChannelForm = { name: '', sound: '', group: '', icon: '', level: '', device_keys: [] }

function Channels() {
  const [channels, setChannels] = useState<Channel[]>([])
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [form, setForm] = useState<ChannelForm>(emptyForm)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    loadData()
  }, [])

  async function loadData() {
    setLoading(true)
    setError('')
    try {
      const [chRes, devRes] = await Promise.all([fetchChannels(), fetchDevices()])
      setChannels(chRes.data || [])
      setDevices(devRes.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  function startEdit(ch: Channel) {
    setEditingId(ch.id)
    setForm({
      name: ch.name,
      sound: ch.sound || '',
      group: ch.group || '',
      icon: ch.icon || '',
      level: ch.level || '',
      device_keys: ch.device_keys || [],
    })
  }

  function cancelEdit() {
    setEditingId(null)
    setForm(emptyForm)
  }

  async function handleSubmit() {
    if (!form.name.trim()) return
    setSubmitting(true)
    setError('')
    try {
      const data: Partial<Channel> = {
        name: form.name.trim(),
        sound: form.sound,
        group: form.group,
        icon: form.icon,
        level: form.level,
        device_keys: form.device_keys,
      }
      if (editingId) {
        await updateChannel(editingId, data)
      } else {
        await createChannel(data)
      }
      setForm(emptyForm)
      setEditingId(null)
      await loadData()
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: string) {
    if (!confirm('确定要删除此频道吗？')) return
    try {
      await deleteChannel(id)
      await loadData()
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败')
    }
  }

  function toggleDeviceKey(key: string) {
    setForm((prev) => ({
      ...prev,
      device_keys: prev.device_keys.includes(key)
        ? prev.device_keys.filter((k) => k !== key)
        : [...prev.device_keys, key],
    }))
  }

  return (
    <div className="page">
      <h2 className="page-title">频道管理</h2>
      <p className="page-description">管理推送频道，配置声音、分组、优先级和目标设备</p>

      {error && <div className="error-banner">{error}</div>}

      <div className="form-card">
        <div className="form-title">{editingId ? '编辑频道' : '创建频道'}</div>
        <div className="form-grid">
          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="ch-name">频道名称 *</label>
              <input
                id="ch-name"
                type="text"
                className="text-input"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="如：alerts、daily-report"
              />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="ch-group">分组</label>
              <input
                id="ch-group"
                type="text"
                className="text-input"
                value={form.group}
                onChange={(e) => setForm({ ...form, group: e.target.value })}
                placeholder="通知分组名称"
              />
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="ch-sound">声音</label>
              <select
                id="ch-sound"
                className="select-input"
                value={form.sound}
                onChange={(e) => setForm({ ...form, sound: e.target.value })}
              >
                {SOUND_PRESETS.map((s) => (
                  <option key={s.value} value={s.value}>{s.label}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="ch-level">优先级</label>
              <select
                id="ch-level"
                className="select-input"
                value={form.level}
                onChange={(e) => setForm({ ...form, level: e.target.value })}
              >
                {LEVEL_OPTIONS.map((l) => (
                  <option key={l.value} value={l.value}>{l.label}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="ch-icon">图标 URL</label>
            <input
              id="ch-icon"
              type="url"
              className="text-input"
              value={form.icon}
              onChange={(e) => setForm({ ...form, icon: e.target.value })}
              placeholder="https://example.com/icon.png"
            />
          </div>

          {devices.length > 0 && (
            <div className="form-group">
              <label className="form-label">目标设备</label>
              <div className="device-checkbox-list">
                {devices.map((d) => (
                  <label key={d.id} className="form-label--checkbox">
                    <input
                      type="checkbox"
                      checked={form.device_keys.includes(d.device_key)}
                      onChange={() => toggleDeviceKey(d.device_key)}
                    />
                    <span>{d.name}</span>
                    {d.is_default && <span className="device-badge">默认</span>}
                  </label>
                ))}
              </div>
              <small className="form-hint">不选择则推送到所有设备</small>
            </div>
          )}

          <div className="form-actions">
            <button
              className="btn btn--primary"
              onClick={handleSubmit}
              disabled={submitting || !form.name.trim()}
            >
              {submitting ? '提交中...' : editingId ? '保存修改' : '创建频道'}
            </button>
            {editingId && (
              <button className="btn btn--secondary" onClick={cancelEdit}>
                取消
              </button>
            )}
          </div>
        </div>
      </div>

      <div className="section">
        <h3 className="section-title">已有频道</h3>
        {loading ? (
          <div className="placeholder-content">加载中...</div>
        ) : channels.length === 0 ? (
          <div className="placeholder-content">暂无频道，请创建第一个频道</div>
        ) : (
          <div className="channel-list">
            {channels.map((ch) => (
              <div key={ch.id} className="channel-card">
                <div className="channel-card-header">
                  <div className="channel-card-name">{ch.name}</div>
                  <div className="channel-card-actions">
                    <button className="btn btn--secondary btn--sm" onClick={() => startEdit(ch)}>
                      编辑
                    </button>
                    <button className="btn btn--danger btn--sm" onClick={() => handleDelete(ch.id)}>
                      删除
                    </button>
                  </div>
                </div>
                <div className="channel-card-meta">
                  {ch.sound && <span>🔊 {ch.sound}</span>}
                  {ch.level && <span>📶 {ch.level}</span>}
                  {ch.group && <span>📁 {ch.group}</span>}
                  {ch.device_keys && ch.device_keys.length > 0 && (
                    <span>📱 {ch.device_keys.length} 台设备</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export default Channels
