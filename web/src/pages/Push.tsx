import { useState } from 'react'
import { sendPush, sendTestPush, PushRequest } from '../api'
import './Pages.css'

interface PushResultDisplay {
  success: boolean
  message: string
  id?: string
}

function Push() {
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [channel, setChannel] = useState('')
  const [extraKey, setExtraKey] = useState('')
  const [extraValue, setExtraValue] = useState('')
  const [extras, setExtras] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<PushResultDisplay | null>(null)

  function addExtra() {
    if (extraKey.trim()) {
      setExtras((prev) => ({ ...prev, [extraKey.trim()]: extraValue }))
      setExtraKey('')
      setExtraValue('')
    }
  }

  function removeExtra(key: string) {
    setExtras((prev) => {
      const next = { ...prev }
      delete next[key]
      return next
    })
  }

  function buildRequest(): PushRequest {
    const req: PushRequest = { title, body }
    if (channel) req.channel = channel
    if (Object.keys(extras).length > 0) req.extra = extras
    return req
  }

  async function handleSend() {
    setSubmitting(true)
    setResult(null)
    try {
      const res = await sendPush(buildRequest())
      setResult({ success: true, message: '推送成功', id: res.data.id })
    } catch (err) {
      setResult({ success: false, message: err instanceof Error ? err.message : '推送失败' })
    } finally {
      setSubmitting(false)
    }
  }

  async function handleTest() {
    setSubmitting(true)
    setResult(null)
    try {
      const res = await sendTestPush(buildRequest())
      setResult({ success: true, message: '测试推送成功（不记录历史）', id: res.data.id })
    } catch (err) {
      setResult({ success: false, message: err instanceof Error ? err.message : '测试推送失败' })
    } finally {
      setSubmitting(false)
    }
  }

  function handleReset() {
    setTitle('')
    setBody('')
    setChannel('')
    setExtras({})
    setExtraKey('')
    setExtraValue('')
    setResult(null)
  }

  return (
    <div className="page">
      <h2 className="page-title">发送推送</h2>
      <p className="page-description">手动发送即时推送通知</p>

      <div className="form-card">
        <div className="form-grid">
          <div className="form-group">
            <label className="form-label" htmlFor="push-title">标题 *</label>
            <input
              id="push-title"
              type="text"
              className="text-input"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="推送标题"
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="push-body">内容 *</label>
            <textarea
              id="push-body"
              className="text-input textarea-input"
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="推送内容"
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="push-channel">频道（可选）</label>
            <input
              id="push-channel"
              type="text"
              className="text-input"
              value={channel}
              onChange={(e) => setChannel(e.target.value)}
              placeholder="留空使用默认频道"
            />
          </div>

          <div className="form-group">
            <label className="form-label">附加字段</label>
            <div className="extra-fields">
              {Object.entries(extras).map(([k, v]) => (
                <div key={k} className="extra-item">
                  <span className="extra-key">{k}</span>
                  <span className="extra-value">{v}</span>
                  <button
                    type="button"
                    className="btn btn--danger btn--sm"
                    onClick={() => removeExtra(k)}
                    aria-label={`删除 ${k}`}
                  >
                    ×
                  </button>
                </div>
              ))}
              <div className="extra-add">
                <input
                  type="text"
                  className="text-input text-input--sm"
                  value={extraKey}
                  onChange={(e) => setExtraKey(e.target.value)}
                  placeholder="键"
                  aria-label="附加字段键"
                />
                <input
                  type="text"
                  className="text-input text-input--sm"
                  value={extraValue}
                  onChange={(e) => setExtraValue(e.target.value)}
                  placeholder="值"
                  aria-label="附加字段值"
                />
                <button type="button" className="btn btn--secondary btn--sm" onClick={addExtra}>
                  添加
                </button>
              </div>
            </div>
          </div>

          <div className="form-actions">
            <button
              className="btn btn--primary"
              onClick={handleSend}
              disabled={submitting || !title || !body}
            >
              {submitting ? '发送中...' : '发送推送'}
            </button>
            <button
              className="btn btn--secondary"
              onClick={handleTest}
              disabled={submitting || !title || !body}
            >
              测试推送
            </button>
            <button className="btn btn--secondary" onClick={handleReset}>
              重置
            </button>
          </div>
        </div>
      </div>

      {result && (
        <div className={`result-card ${result.success ? 'result-card--success' : 'result-card--error'}`}>
          <div className="result-icon">{result.success ? '✓' : '✗'}</div>
          <div className="result-content">
            <div className="result-message">{result.message}</div>
            {result.id && <div className="result-id">消息 ID: {result.id}</div>}
          </div>
        </div>
      )}
    </div>
  )
}

export default Push
