import { useState, useEffect } from 'react'
import { fetchChannels, createChannel, updateChannel, deleteChannel, Channel } from '../api'
import './Pages.css'

const LEVEL_OPTIONS = [
  { value: 'active', label: 'Active - 普通通知' },
  { value: 'timeSensitive', label: 'Time Sensitive - 时效性通知' },
  { value: 'passive', label: 'Passive - 静默通知' },
  { value: 'critical', label: 'Critical - 紧急通知' },
]

const SOUND_PRESETS = [
  { value: '', label: '默认声音' },
  { value: 'alarm.caf', label: '🚨 alarm - 警报' },
  { value: 'bell.caf', label: '🔔 bell - 铃声' },
  { value: 'calypso.caf', label: '🎵 calypso' },
  { value: 'chime.caf', label: '🎐 chime - 风铃' },
  { value: 'electronic.caf', label: '⚡ electronic' },
  { value: 'fanfare.caf', label: '🎺 fanfare - 号角' },
  { value: 'glass.caf', label: '🥂 glass - 玻璃' },
  { value: 'healthnotification.caf', label: '❤️ health' },
  { value: 'minuet.caf', label: '🎼 minuet - 小步舞曲' },
  { value: 'newsflash.caf', label: '📰 newsflash' },
  { value: 'paymentsuccess.caf', label: '💰 paymentsuccess' },
  { value: 'shake.caf', label: '📳 shake - 震动' },
  { value: 'silence.caf', label: '🔇 silence - 静音' },
  { value: 'update.caf', label: '🔄 update - 更新' },
]

interface ChannelForm {
  name: string
  sound: string
  group: string
  icon: string
  level: string
}

const emptyForm: ChannelForm = { name: '', sound: '', group: '', icon: '', level: 'active' }

function Channels() {
  const [channels, setChannels] = useState<Channel[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<ChannelForm>(emptyForm)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')

  useEffect(() => { loadData() }, [])

  async function loadData() {
    setLoading(true)
    setError('')
    try {
      const res = await fetchChannels()
      setChannels(res.data || [])
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
      level: ch.level || 'active',
    })
    setShowForm(true)
    setFormError('')
  }

  function cancelForm() {
    setShowForm(false)
    setEditingId(null)
    setForm(emptyForm)
    setFormError('')
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    setSubmitting(true)
    setFormError('')
    try {
      const data: Partial<Channel> = {
        name: form.name.trim(),
        sound: form.sound,
        group: form.group,
        icon: form.icon,
        level: form.level,
      }
      if (editingId) {
        await updateChannel(editingId, data)
      } else {
        await createChannel(data)
      }
      cancelForm()
      await loadData()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: string, name: string) {
    if (name === 'default') {
      setError('默认频道不能删除')
      return
    }
    if (!confirm(`确定要删除频道「${name}」吗？`)) return
    try {
      await deleteChannel(id)
      await loadData()
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败')
    }
  }

  function getLevelLabel(level: string): string {
    const opt = LEVEL_OPTIONS.find((l) => l.value === level)
    return opt ? opt.label : level
  }

  function getSoundLabel(sound: string): string {
    if (!sound) return '默认'
    const opt = SOUND_PRESETS.find((s) => s.value === sound)
    return opt ? opt.label : sound
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h2 className="page-title">频道管理</h2>
          <p className="page-description">管理推送频道，每个频道绑定独特的声音和优先级，实现"听声辨事"</p>
        </div>
        <button className="btn btn--primary" onClick={() => { if (showForm && !editingId) { cancelForm() } else { cancelForm(); setShowForm(true) } }}>
          {showForm && !editingId ? '取消' : '+ 新建频道'}
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {showForm && (
        <div className="form-card">
          <h3 className="form-title">{editingId ? '编辑频道' : '新建频道'}</h3>
          {formError && <div className="error-banner">{formError}</div>}
          <form onSubmit={handleSubmit} className="form-grid">
            <div className="form-row">
              <div className="form-group">
                <label className="form-label" htmlFor="ch-name">频道名称 *</label>
                <input id="ch-name" type="text" className="text-input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="如：urgent、daily" required disabled={editingId !== null && form.name === 'default'} />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="ch-group">分组</label>
                <input id="ch-group" type="text" className="text-input" value={form.group} onChange={(e) => setForm({ ...form, group: e.target.value })} placeholder="通知分组名称" />
              </div>
            </div>
            <div className="form-row">
              <div className="form-group">
                <label className="form-label" htmlFor="ch-sound">声音</label>
                <select id="ch-sound" className="select-input" value={form.sound} onChange={(e) => setForm({ ...form, sound: e.target.value })}>
                  {SOUND_PRESETS.map((s) => (<option key={s.value} value={s.value}>{s.label}</option>))}
                </select>
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="ch-level">优先级</label>
                <select id="ch-level" className="select-input" value={form.level} onChange={(e) => setForm({ ...form, level: e.target.value })}>
                  {LEVEL_OPTIONS.map((l) => (<option key={l.value} value={l.value}>{l.label}</option>))}
                </select>
              </div>
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="ch-icon">图标 URL</label>
              <input id="ch-icon" type="url" className="text-input" value={form.icon} onChange={(e) => setForm({ ...form, icon: e.target.value })} placeholder="https://example.com/icon.png（可选）" />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn btn--primary" disabled={submitting || !form.name.trim()}>
                {submitting ? '提交中...' : editingId ? '保存修改' : '创建频道'}
              </button>
              <button type="button" className="btn btn--secondary" onClick={cancelForm}>取消</button>
            </div>
          </form>
        </div>
      )}

      {loading ? (
        <div className="placeholder-content"><p>加载中...</p></div>
      ) : channels.length === 0 ? (
        <div className="placeholder-content">
          <p>暂无频道</p>
          <p className="placeholder-hint">创建频道来为不同类型的通知绑定声音和优先级</p>
        </div>
      ) : (
        <div className="channel-list">
          {channels.map((ch) => (
            <div key={ch.id} className={`channel-card ${ch.name === 'default' ? 'channel-card--default' : ''}`}>
              <div className="channel-card-header">
                <div className="channel-card-name">
                  {ch.name}
                  {ch.name === 'default' && <span className="device-badge">默认</span>}
                </div>
                <div className="channel-card-actions">
                  <button className="btn btn--secondary btn--sm" onClick={() => startEdit(ch)}>编辑</button>
                  {ch.name !== 'default' && (
                    <button className="btn btn--danger btn--sm" onClick={() => handleDelete(ch.id, ch.name)}>删除</button>
                  )}
                </div>
              </div>
              <div className="channel-card-meta">
                <span>🔊 {getSoundLabel(ch.sound)}</span>
                <span>📶 {getLevelLabel(ch.level)}</span>
                {ch.group && <span>📁 {ch.group}</span>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default Channels
