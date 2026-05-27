import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { fetchReminders, createReminder, updateReminder, cancelReminder, fetchChannels, fetchDevices, parseSchedule, Reminder, CreateReminderRequest, Channel, Device, ScheduleParseResult } from '../api'
import './Pages.css'

type TabStatus = '' | 'active' | 'completed' | 'cancelled'

function Reminders() {
  const [reminders, setReminders] = useState<Reminder[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [activeTab, setActiveTab] = useState<TabStatus>('active')
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)

  // Form state
  const [formTitle, setFormTitle] = useState('')
  const [formBody, setFormBody] = useState('')
  const [formTriggerAt, setFormTriggerAt] = useState('')
  const [formChannel, setFormChannel] = useState('')
  const [formRepeat, setFormRepeat] = useState('once')
  const [formRepeatRule, setFormRepeatRule] = useState('')
  const [formDeviceKeys, setFormDeviceKeys] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')

  // Natural language input
  const [nlInput, setNlInput] = useState('')
  const [nlParsing, setNlParsing] = useState(false)
  const [nlResult, setNlResult] = useState<ScheduleParseResult | null>(null)
  const [nlError, setNlError] = useState('')
  const [useNlMode, setUseNlMode] = useState(false)

  useEffect(() => { loadReminders() }, [activeTab])
  useEffect(() => {
    loadChannels()
    loadDevices()
  }, [])

  async function loadReminders() {
    setLoading(true)
    setError('')
    try {
      const res = await fetchReminders(activeTab || undefined)
      setReminders(res.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载提醒失败')
    } finally {
      setLoading(false)
    }
  }

  async function loadChannels() {
    try {
      const res = await fetchChannels()
      setChannels(res.data || [])
    } catch { /* ignore */ }
  }

  async function loadDevices() {
    try {
      const res = await fetchDevices()
      setDevices(res.data || [])
    } catch { /* ignore */ }
  }

  function startEdit(r: Reminder) {
    setEditingId(r.id)
    setFormTitle(r.title)
    setFormBody(r.body || '')
    setFormTriggerAt(new Date(r.trigger_at).toISOString().slice(0, 16))
    setFormChannel(r.channel)
    setFormRepeat(r.repeat_type)
    setFormRepeatRule(r.repeat_rule || '')
    setFormDeviceKeys([])
    setShowForm(true)
    setFormError('')
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setFormError('')
    try {
      const data: CreateReminderRequest = {
        title: formTitle,
        trigger_at: new Date(formTriggerAt).toISOString(),
      }
      if (formBody) data.body = formBody
      if (formChannel) data.channel = formChannel
      if (formRepeat !== 'once') data.repeat = formRepeat
      if (formRepeatRule) data.repeat_rule = formRepeatRule
      // TODO: send device_keys when backend supports it

      if (editingId) {
        await updateReminder(editingId, data)
      } else {
        await createReminder(data)
      }
      resetForm()
      setShowForm(false)
      loadReminders()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleCancel(id: string) {
    if (!confirm('确定要取消此提醒吗？')) return
    try {
      await cancelReminder(id)
      loadReminders()
    } catch (err) {
      setError(err instanceof Error ? err.message : '取消提醒失败')
    }
  }

  async function handleReactivate(r: Reminder) {
    try {
      const triggerAt = new Date(r.trigger_at)
      const now = new Date()
      const newTrigger = triggerAt > now ? triggerAt.toISOString() : new Date(now.getTime() + 3600000).toISOString()
      await updateReminder(r.id, {
        title: r.title,
        body: r.body,
        trigger_at: newTrigger,
        channel: r.channel,
        repeat: r.repeat_type,
        repeat_rule: r.repeat_rule,
      })
      loadReminders()
    } catch (err) {
      setError(err instanceof Error ? err.message : '重新激活失败')
    }
  }

  function resetForm() {
    setFormTitle('')
    setFormBody('')
    setFormTriggerAt('')
    setFormChannel('')
    setFormRepeat('once')
    setFormRepeatRule('')
    setFormDeviceKeys([])
    setFormError('')
    setEditingId(null)
    setNlInput('')
    setNlResult(null)
    setNlError('')
  }

  async function handleNlParse() {
    if (!nlInput.trim()) return
    setNlParsing(true)
    setNlError('')
    setNlResult(null)
    try {
      const res = await parseSchedule(nlInput.trim())
      const result = res.data
      setNlResult(result)
      // Auto-fill the form with parsed result
      if (result.title) setFormTitle(result.title)
      if (result.schedule) {
        const sc = result.schedule
        setFormRepeat(sc.type)
        // Convert schedule config to form fields
        if (sc.type === 'once' && sc.config.datetime) {
          const dt = new Date(sc.config.datetime as string)
          setFormTriggerAt(dt.toISOString().slice(0, 16))
        } else if (sc.config.time) {
          // For daily/weekly/workday/weekend, set trigger time to today + that time
          const timeStr = sc.config.time as string
          const [h, m] = timeStr.split(':')
          const now = new Date()
          now.setHours(parseInt(h), parseInt(m), 0, 0)
          if (now < new Date()) now.setDate(now.getDate() + 1)
          setFormTriggerAt(now.toISOString().slice(0, 16))
        }
        if (sc.type === 'weekly' && sc.config.days) {
          setFormRepeatRule(JSON.stringify(sc.config.days))
        }
        if (sc.type === 'crontab' && sc.config.expression) {
          setFormRepeatRule(sc.config.expression as string)
        }
      }
    } catch (err) {
      setNlError(err instanceof Error ? err.message : '解析失败')
    } finally {
      setNlParsing(false)
    }
  }

  function formatTime(iso: string): string {
    return new Date(iso).toLocaleString('zh-CN')
  }

  const tabs: { label: string; value: TabStatus }[] = [
    { label: '活跃', value: 'active' },
    { label: '已完成', value: 'completed' },
    { label: '已取消', value: 'cancelled' },
    { label: '全部', value: '' },
  ]

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h2 className="page-title">提醒管理</h2>
          <p className="page-description">创建和管理定时提醒</p>
        </div>
        <button className="btn btn--primary" onClick={() => { if (showForm && !editingId) { resetForm(); setShowForm(false) } else { resetForm(); setShowForm(true) } }}>
          {showForm && !editingId ? '取消' : '+ 新建提醒'}
        </button>
      </div>

      {showForm && (
        <div className="form-card">
          <h3 className="form-title">{editingId ? '编辑提醒' : '创建提醒'}</h3>

          {/* Natural language input toggle */}
          {!editingId && (
            <div style={{ marginBottom: '1rem' }}>
              <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem' }}>
                <button type="button" className={`btn btn--sm ${!useNlMode ? 'btn--primary' : 'btn--secondary'}`} onClick={() => setUseNlMode(false)}>手动配置</button>
                <button type="button" className={`btn btn--sm ${useNlMode ? 'btn--primary' : 'btn--secondary'}`} onClick={() => setUseNlMode(true)}>✨ 自然语言</button>
              </div>
              {useNlMode && (
                <div className="form-group">
                  <label className="form-label">用自然语言描述你的提醒</label>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <input
                      type="text"
                      className="text-input"
                      value={nlInput}
                      onChange={(e) => setNlInput(e.target.value)}
                      placeholder="如：每天早上9点提醒我开会、下周一下午3点提醒我交报告"
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleNlParse() } }}
                      style={{ flex: 1 }}
                    />
                    <button type="button" className="btn btn--primary" onClick={handleNlParse} disabled={nlParsing || !nlInput.trim()}>
                      {nlParsing ? '解析中...' : '解析'}
                    </button>
                  </div>
                  {nlError && <div className="error-banner" style={{ marginTop: '0.5rem' }}>{nlError}</div>}
                  {nlResult && (
                    <div className="result-card result-card--success" style={{ marginTop: '0.5rem' }}>
                      <div className="result-content">
                        <div className="result-message">✓ 解析成功</div>
                        <div style={{ fontSize: '0.85rem', color: '#374151', marginTop: '0.25rem' }}>
                          标题: {nlResult.title} | 类型: {nlResult.schedule.type} | 配置: {JSON.stringify(nlResult.schedule.config)}
                        </div>
                      </div>
                    </div>
                  )}
                  <small className="form-hint">输入自然语言描述，AI 会自动解析为定时配置并填充下方表单</small>
                </div>
              )}
            </div>
          )}

          {formError && <div className="error-banner">{formError}</div>}
          <form onSubmit={handleSubmit} className="form-grid">
            <div className="form-group">
              <label className="form-label" htmlFor="reminder-title">标题 *</label>
              <input id="reminder-title" type="text" className="text-input" value={formTitle} onChange={(e) => setFormTitle(e.target.value)} required maxLength={200} placeholder="提醒标题" />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="reminder-body">内容</label>
              <textarea id="reminder-body" className="text-input textarea-input" value={formBody} onChange={(e) => setFormBody(e.target.value)} maxLength={4000} placeholder="提醒内容（可选）" />
            </div>
            <div className="form-row">
              <div className="form-group">
                <label className="form-label" htmlFor="reminder-trigger">触发时间 *</label>
                <input id="reminder-trigger" type="datetime-local" className="text-input" value={formTriggerAt} onChange={(e) => setFormTriggerAt(e.target.value)} required />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="reminder-channel">频道</label>
                <select id="reminder-channel" className="select-input" value={formChannel} onChange={(e) => setFormChannel(e.target.value)}>
                  <option value="">默认</option>
                  {channels.map((ch) => (<option key={ch.id} value={ch.name}>{ch.name}</option>))}
                </select>
              </div>
            </div>
            <div className="form-row">
              <div className="form-group">
                <label className="form-label" htmlFor="reminder-repeat">重复类型</label>
                <select id="reminder-repeat" className="select-input" value={formRepeat} onChange={(e) => setFormRepeat(e.target.value)}>
                  <option value="once">一次性</option>
                  <option value="daily">每天</option>
                  <option value="weekly">每周</option>
                  <option value="workday">工作日</option>
                  <option value="weekend">周末</option>
                  <option value="monthly">每月</option>
                  <option value="yearly">每年</option>
                  <option value="interval">固定间隔</option>
                  <option value="crontab">Cron 表达式</option>
                </select>
              </div>
              {(formRepeat === 'weekly' || formRepeat === 'cron') && (
                <div className="form-group">
                  <label className="form-label" htmlFor="reminder-rule">重复规则</label>
                  <input id="reminder-rule" type="text" className="text-input" value={formRepeatRule} onChange={(e) => setFormRepeatRule(e.target.value)} placeholder={formRepeat === 'weekly' ? '0-6 (周日-周六)' : 'cron 表达式'} />
                </div>
              )}
            </div>

            {/* Device selection */}
            {devices.length > 0 && (
              <div className="form-group">
                <label className="form-label">目标设备</label>
                <div className="device-checkbox-list">
                  {devices.map((d) => (
                    <label key={d.id} className="form-label--checkbox">
                      <input
                        type="checkbox"
                        checked={formDeviceKeys.includes(d.device_key)}
                        onChange={() => {
                          setFormDeviceKeys((prev) =>
                            prev.includes(d.device_key)
                              ? prev.filter((k) => k !== d.device_key)
                              : [...prev, d.device_key]
                          )
                        }}
                      />
                      <span>{d.name}</span>
                      {d.is_default && <span className="device-badge">默认</span>}
                    </label>
                  ))}
                </div>
                <small className="form-hint">选择提醒推送的目标设备，不选择则推送到所有设备</small>
              </div>
            )}

            <div className="form-actions">
              <button type="submit" className="btn btn--primary" disabled={submitting}>
                {submitting ? '提交中...' : editingId ? '保存修改' : '创建提醒'}
              </button>
              <button type="button" className="btn btn--secondary" onClick={() => { resetForm(); setShowForm(false) }}>取消</button>
            </div>
          </form>
        </div>
      )}

      <div className="tabs">
        {tabs.map((tab) => (
          <button key={tab.value} className={`tab-btn ${activeTab === tab.value ? 'tab-btn--active' : ''}`} onClick={() => setActiveTab(tab.value)}>
            {tab.label}
          </button>
        ))}
      </div>

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="placeholder-content"><p>加载中...</p></div>
      ) : reminders.length === 0 ? (
        <div className="placeholder-content"><p>暂无提醒</p></div>
      ) : (
        <div className="reminder-list">
          {reminders.map((r) => (
            <div key={r.id} className="reminder-card">
              <div className="reminder-header">
                <h4 className="reminder-title">{r.title}</h4>
                <span className={`status-badge status-badge--${r.status === 'active' ? 'success' : r.status === 'cancelled' ? 'danger' : 'pending'}`}>
                  {r.status === 'active' ? '活跃' : r.status === 'cancelled' ? '已取消' : '已完成'}
                </span>
              </div>
              {r.body && <p className="reminder-body">{r.body}</p>}
              <div className="reminder-meta">
                <span>频道: {r.channel}</span>
                <span>触发: {formatTime(r.trigger_at)}</span>
                <span>重复: {r.repeat_type === 'once' ? '一次性' : r.repeat_type === 'daily' ? '每天' : r.repeat_type === 'weekly' ? '每周' : r.repeat_type}</span>
                {r.next_trigger && <span>下次: {formatTime(r.next_trigger)}</span>}
                <Link to={`/messages?channel=${encodeURIComponent(r.channel)}`} className="reminder-push-count">
                  已推送: 0 次
                </Link>
              </div>
              <div className="reminder-actions">
                {r.status === 'active' && (
                  <>
                    <button className="btn btn--secondary btn--sm" onClick={() => startEdit(r)}>编辑</button>
                    <button className="btn btn--danger btn--sm" onClick={() => handleCancel(r.id)}>取消</button>
                  </>
                )}
                {r.status === 'cancelled' && (
                  <button className="btn btn--primary btn--sm" onClick={() => handleReactivate(r)}>重新激活</button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default Reminders
