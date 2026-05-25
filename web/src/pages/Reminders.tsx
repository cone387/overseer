import { useEffect, useState } from 'react'
import { fetchReminders, createReminder, cancelReminder, Reminder, CreateReminderRequest } from '../api'
import './Pages.css'

type TabStatus = '' | 'active' | 'completed' | 'cancelled'

function Reminders() {
  const [reminders, setReminders] = useState<Reminder[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [activeTab, setActiveTab] = useState<TabStatus>('active')
  const [showForm, setShowForm] = useState(false)

  // Form state
  const [formTitle, setFormTitle] = useState('')
  const [formBody, setFormBody] = useState('')
  const [formTriggerAt, setFormTriggerAt] = useState('')
  const [formChannel, setFormChannel] = useState('')
  const [formRepeat, setFormRepeat] = useState('once')
  const [formRepeatRule, setFormRepeatRule] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')

  useEffect(() => {
    loadReminders()
  }, [activeTab])

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

  async function handleCreate(e: React.FormEvent) {
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

      await createReminder(data)
      resetForm()
      setShowForm(false)
      loadReminders()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '创建提醒失败')
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

  function resetForm() {
    setFormTitle('')
    setFormBody('')
    setFormTriggerAt('')
    setFormChannel('')
    setFormRepeat('once')
    setFormRepeatRule('')
    setFormError('')
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
        <button className="btn btn--primary" onClick={() => setShowForm(!showForm)}>
          {showForm ? '取消' : '+ 新建提醒'}
        </button>
      </div>

      {showForm && (
        <div className="form-card">
          <h3 className="form-title">创建提醒</h3>
          {formError && <div className="error-banner">{formError}</div>}
          <form onSubmit={handleCreate} className="form-grid">
            <div className="form-group">
              <label className="form-label" htmlFor="reminder-title">标题 *</label>
              <input
                id="reminder-title"
                type="text"
                className="text-input"
                value={formTitle}
                onChange={(e) => setFormTitle(e.target.value)}
                required
                maxLength={200}
                placeholder="提醒标题"
              />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="reminder-body">内容</label>
              <textarea
                id="reminder-body"
                className="text-input textarea-input"
                value={formBody}
                onChange={(e) => setFormBody(e.target.value)}
                maxLength={4000}
                placeholder="提醒内容（可选）"
              />
            </div>
            <div className="form-row">
              <div className="form-group">
                <label className="form-label" htmlFor="reminder-trigger">触发时间 *</label>
                <input
                  id="reminder-trigger"
                  type="datetime-local"
                  className="text-input"
                  value={formTriggerAt}
                  onChange={(e) => setFormTriggerAt(e.target.value)}
                  required
                />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="reminder-channel">频道</label>
                <input
                  id="reminder-channel"
                  type="text"
                  className="text-input"
                  value={formChannel}
                  onChange={(e) => setFormChannel(e.target.value)}
                  placeholder="default"
                />
              </div>
            </div>
            <div className="form-row">
              <div className="form-group">
                <label className="form-label" htmlFor="reminder-repeat">重复类型</label>
                <select
                  id="reminder-repeat"
                  className="select-input"
                  value={formRepeat}
                  onChange={(e) => setFormRepeat(e.target.value)}
                >
                  <option value="once">一次性</option>
                  <option value="daily">每天</option>
                  <option value="weekly">每周</option>
                  <option value="cron">Cron 表达式</option>
                </select>
              </div>
              {(formRepeat === 'weekly' || formRepeat === 'cron') && (
                <div className="form-group">
                  <label className="form-label" htmlFor="reminder-rule">重复规则</label>
                  <input
                    id="reminder-rule"
                    type="text"
                    className="text-input"
                    value={formRepeatRule}
                    onChange={(e) => setFormRepeatRule(e.target.value)}
                    placeholder={formRepeat === 'weekly' ? '0-6 (周日-周六)' : 'cron 表达式'}
                  />
                </div>
              )}
            </div>
            <div className="form-actions">
              <button type="submit" className="btn btn--primary" disabled={submitting}>
                {submitting ? '创建中...' : '创建提醒'}
              </button>
              <button type="button" className="btn btn--secondary" onClick={() => { resetForm(); setShowForm(false) }}>
                取消
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="tabs">
        {tabs.map((tab) => (
          <button
            key={tab.value}
            className={`tab-btn ${activeTab === tab.value ? 'tab-btn--active' : ''}`}
            onClick={() => setActiveTab(tab.value)}
          >
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
                  {r.status}
                </span>
              </div>
              {r.body && <p className="reminder-body">{r.body}</p>}
              <div className="reminder-meta">
                <span>频道: {r.channel}</span>
                <span>触发: {formatTime(r.trigger_at)}</span>
                <span>重复: {r.repeat_type}</span>
                {r.next_trigger && <span>下次: {formatTime(r.next_trigger)}</span>}
              </div>
              {r.status === 'active' && (
                <div className="reminder-actions">
                  <button
                    className="btn btn--danger btn--sm"
                    onClick={() => handleCancel(r.id)}
                  >
                    取消提醒
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default Reminders
