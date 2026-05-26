import { useState, useEffect } from 'react'
import { sendPush, PushRequest, fetchDevices, Device } from '../api'
import './Pages.css'

interface PushResultDisplay {
  success: boolean
  message: string
  id?: string
}

const LEVEL_OPTIONS = [
  { value: '', label: '默认（跟随频道配置）' },
  { value: 'active', label: 'Active - 普通通知' },
  { value: 'timeSensitive', label: 'Time Sensitive - 时效性通知' },
  { value: 'passive', label: 'Passive - 静默通知' },
  { value: 'critical', label: 'Critical - 紧急通知（即使静音也响）' },
]

const SOUND_PRESETS = [
  { value: '', label: '默认（跟随频道配置）' },
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

function Push() {
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [channel, setChannel] = useState('')
  const [sound, setSound] = useState('')
  const [customSound, setCustomSound] = useState('')
  const [icon, setIcon] = useState('')
  const [group, setGroup] = useState('')
  const [level, setLevel] = useState('')
  const [url, setUrl] = useState('')
  const [extraKey, setExtraKey] = useState('')
  const [extraValue, setExtraValue] = useState('')
  const [extras, setExtras] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<PushResultDisplay | null>(null)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [devices, setDevices] = useState<Device[]>([])
  const [selectedDeviceKeys, setSelectedDeviceKeys] = useState<string[]>([])

  useEffect(() => {
    fetchDevices().then((res) => {
      const devs = res.data || []
      setDevices(devs)
      const defaultKeys = devs.filter((d) => d.is_default).map((d) => d.device_key)
      setSelectedDeviceKeys(defaultKeys)
    }).catch(() => {})
  }, [])

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
    const effectiveSound = customSound || sound
    if (effectiveSound) req.sound = effectiveSound
    if (icon) req.icon = icon
    if (group) req.group = group
    if (level) req.level = level
    if (url) req.url = url
    if (Object.keys(extras).length > 0) req.extra = extras
    if (selectedDeviceKeys.length > 0) req.device_keys = selectedDeviceKeys
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

  function handleReset() {
    setTitle('')
    setBody('')
    setChannel('')
    setSound('')
    setCustomSound('')
    setIcon('')
    setGroup('')
    setLevel('')
    setUrl('')
    setExtras({})
    setExtraKey('')
    setExtraValue('')
    setResult(null)
    const defaultKeys = devices.filter((d) => d.is_default).map((d) => d.device_key)
    setSelectedDeviceKeys(defaultKeys)
  }

  return (
    <div className="page">
      <h2 className="page-title">发送推送</h2>
      <p className="page-description">手动发送即时推送通知，支持自定义声音、图标、优先级等 Bark 参数</p>

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

          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="push-channel">频道</label>
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
              <label className="form-label" htmlFor="push-group">分组</label>
              <input
                id="push-group"
                type="text"
                className="text-input"
                value={group}
                onChange={(e) => setGroup(e.target.value)}
                placeholder="通知分组名称"
              />
            </div>
          </div>

          <div className="form-divider">
            <button
              type="button"
              className="btn btn--link"
              onClick={() => setShowAdvanced(!showAdvanced)}
            >
              {showAdvanced ? '▼ 收起推送配置' : '▶ 展开推送配置（声音、图标、优先级...）'}
            </button>
          </div>

          {showAdvanced && (
            <>
              <div className="form-group">
                <label className="form-label" htmlFor="push-sound">推送声音</label>
                <select
                  id="push-sound"
                  className="select-input"
                  value={sound}
                  onChange={(e) => { setSound(e.target.value); setCustomSound('') }}
                >
                  {SOUND_PRESETS.map((s) => (
                    <option key={s.value} value={s.value}>{s.label}</option>
                  ))}
                  <option value="__custom__">自定义声音文件...</option>
                </select>
                {sound === '__custom__' && (
                  <input
                    type="text"
                    className="text-input"
                    style={{ marginTop: '0.5rem' }}
                    value={customSound}
                    onChange={(e) => setCustomSound(e.target.value)}
                    placeholder="输入自定义声音文件名（如 mysound.caf）"
                  />
                )}
                <small className="form-hint">选择推送到达时播放的声音，自定义声音需先导入 Bark App</small>
              </div>

              <div className="form-row">
                <div className="form-group">
                  <label className="form-label" htmlFor="push-level">优先级</label>
                  <select
                    id="push-level"
                    className="select-input"
                    value={level}
                    onChange={(e) => setLevel(e.target.value)}
                  >
                    {LEVEL_OPTIONS.map((l) => (
                      <option key={l.value} value={l.value}>{l.label}</option>
                    ))}
                  </select>
                  <small className="form-hint">Critical 级别即使静音也会发出声音</small>
                </div>
                <div className="form-group">
                  <label className="form-label" htmlFor="push-icon">图标 URL</label>
                  <input
                    id="push-icon"
                    type="url"
                    className="text-input"
                    value={icon}
                    onChange={(e) => setIcon(e.target.value)}
                    placeholder="https://example.com/icon.png"
                  />
                  <small className="form-hint">通知图标，支持 URL 格式</small>
                </div>
              </div>

              <div className="form-group">
                <label className="form-label" htmlFor="push-url">点击跳转 URL</label>
                <input
                  id="push-url"
                  type="url"
                  className="text-input"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="https://example.com（点击通知后打开的链接）"
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
            </>
          )}

          {devices.length > 0 && (
            <div className="form-group">
              <label className="form-label">目标设备</label>
              <div className="device-checkbox-list">
                {devices.map((d) => (
                  <label key={d.id} className="form-label--checkbox">
                    <input
                      type="checkbox"
                      checked={selectedDeviceKeys.includes(d.device_key)}
                      onChange={() => {
                        setSelectedDeviceKeys((prev) =>
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
              <small className="form-hint">选择推送的目标设备，不选择则推送到所有设备</small>
            </div>
          )}

          <div className="form-actions">
            <button
              className="btn btn--primary"
              onClick={handleSend}
              disabled={submitting || !title || !body}
            >
              {submitting ? '发送中...' : '发送推送'}
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
