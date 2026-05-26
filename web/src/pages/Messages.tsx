import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { fetchMessages, fetchChannels, Message, MessageFilter, Channel } from '../api'
import './Pages.css'

type TimeRange = '1h' | '24h' | '7d' | '30d' | 'custom'

function toISO(localDatetime: string): string {
  if (!localDatetime) return ''
  return new Date(localDatetime).toISOString()
}

function getFromDate(range: TimeRange): string {
  const d = new Date()
  switch (range) {
    case '1h': d.setHours(d.getHours() - 1); break
    case '24h': d.setHours(d.getHours() - 24); break
    case '7d': d.setDate(d.getDate() - 7); break
    case '30d': d.setDate(d.getDate() - 30); break
    default: return ''
  }
  return d.toISOString()
}

function Messages() {
  const [searchParams] = useSearchParams()
  const [messages, setMessages] = useState<Message[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [channelFilter, setChannelFilter] = useState(() => searchParams.get('channel') || '')
  const [statusFilter, setStatusFilter] = useState('')
  const [timeRange, setTimeRange] = useState<TimeRange>('7d')
  const [customFrom, setCustomFrom] = useState(() => {
    const d = new Date()
    d.setDate(d.getDate() - 7)
    return d.toISOString().slice(0, 16)
  })
  const [customTo, setCustomTo] = useState(() => {
    return new Date().toISOString().slice(0, 16)
  })

  useEffect(() => { loadChannels() }, [])
  useEffect(() => {
    loadMessages()
  }, [page, channelFilter, statusFilter, timeRange, customFrom, customTo])

  async function loadChannels() {
    try {
      const res = await fetchChannels()
      setChannels(res.data || [])
    } catch { /* ignore */ }
  }

  async function loadMessages() {
    setLoading(true)
    setError('')
    try {
      const filter: MessageFilter = {
        page,
        page_size: pageSize,
      }

      if (timeRange === 'custom') {
        if (customFrom) filter.from = toISO(customFrom)
        if (customTo) filter.to = toISO(customTo)
      } else {
        filter.from = getFromDate(timeRange)
        filter.to = new Date().toISOString()
      }

      if (channelFilter) filter.channel = channelFilter
      if (statusFilter) filter.status = statusFilter

      const res = await fetchMessages(filter)
      const data = res.data
      setMessages(data.data || [])
      setTotal(data.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载消息失败')
    } finally {
      setLoading(false)
    }
  }

  const totalPages = Math.ceil(total / pageSize)

  function formatTime(iso: string): string {
    return new Date(iso).toLocaleString('zh-CN')
  }

  function getStatusClass(status: string): string {
    switch (status) {
      case 'success': return 'status-badge--success'
      case 'failed': return 'status-badge--danger'
      default: return 'status-badge--pending'
    }
  }

  function handleReset() {
    setChannelFilter('')
    setStatusFilter('')
    setTimeRange('7d')
    setCustomFrom(() => {
      const d = new Date()
      d.setDate(d.getDate() - 7)
      return d.toISOString().slice(0, 16)
    })
    setCustomTo(new Date().toISOString().slice(0, 16))
    setPage(1)
  }

  const timeRangeOptions: { value: TimeRange; label: string }[] = [
    { value: '1h', label: '最近1小时' },
    { value: '24h', label: '最近24小时' },
    { value: '7d', label: '最近7天' },
    { value: '30d', label: '最近30天' },
    { value: 'custom', label: '自定义' },
  ]

  return (
    <div className="page">
      <h2 className="page-title">推送历史</h2>
      <p className="page-description">查看所有推送消息记录</p>

      <div className="filter-bar" style={{ flexDirection: 'column', alignItems: 'stretch', gap: '0.75rem' }}>
        <div className="filter-row" style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          <span className="form-label" style={{ margin: 0, minWidth: 'fit-content' }}>时间范围:</span>
          <div className="time-range-buttons" style={{ display: 'flex', gap: '0.25rem', flexWrap: 'wrap' }}>
            {timeRangeOptions.map((opt) => (
              <button
                key={opt.value}
                className={`btn btn--sm ${timeRange === opt.value ? 'btn--primary' : 'btn--secondary'}`}
                onClick={() => { setTimeRange(opt.value); setPage(1) }}
              >
                {opt.label}
              </button>
            ))}
          </div>
          {timeRange === 'custom' && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <input
                type="datetime-local"
                className="text-input"
                value={customFrom}
                onChange={(e) => { setCustomFrom(e.target.value); setPage(1) }}
                aria-label="开始时间"
              />
              <span>至</span>
              <input
                type="datetime-local"
                className="text-input"
                value={customTo}
                onChange={(e) => { setCustomTo(e.target.value); setPage(1) }}
                aria-label="结束时间"
              />
            </div>
          )}
        </div>
        <div className="filter-row" style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          <span className="form-label" style={{ margin: 0, minWidth: 'fit-content' }}>频道:</span>
          <select
            className="select-input"
            value={channelFilter}
            onChange={(e) => { setChannelFilter(e.target.value); setPage(1) }}
            aria-label="频道筛选"
          >
            <option value="">全部频道</option>
            {channels.map((ch) => (
              <option key={ch.id} value={ch.name}>{ch.name}</option>
            ))}
          </select>
          <span className="form-label" style={{ margin: 0, minWidth: 'fit-content' }}>状态:</span>
          <select
            className="select-input"
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1) }}
            aria-label="状态筛选"
          >
            <option value="">全部</option>
            <option value="success">成功</option>
            <option value="failed">失败</option>
            <option value="pending">待推送</option>
          </select>
          <button className="btn btn--secondary btn--sm" onClick={handleReset}>重置</button>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>来源</th>
              <th>频道</th>
              <th>标题</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={5} className="table-empty">加载中...</td></tr>
            ) : messages.length === 0 ? (
              <tr><td colSpan={5} className="table-empty">暂无推送记录</td></tr>
            ) : (
              messages.map((msg) => (
                <tr key={msg.id}>
                  <td className="td-nowrap">{formatTime(msg.received_at)}</td>
                  <td>{msg.source}</td>
                  <td><span className="channel-badge">{msg.channel}</span></td>
                  <td className="td-title">{msg.title}</td>
                  <td>
                    <span className={`status-badge ${getStatusClass(msg.status)}`}>
                      {msg.status}
                    </span>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="pagination">
          <button
            className="btn btn--secondary"
            disabled={page <= 1}
            onClick={() => setPage(page - 1)}
          >
            上一页
          </button>
          <span className="pagination-info">
            第 {page} / {totalPages} 页（共 {total} 条）
          </span>
          <button
            className="btn btn--secondary"
            disabled={page >= totalPages}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}

export default Messages
