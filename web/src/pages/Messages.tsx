import { useEffect, useState } from 'react'
import { fetchMessages, Message, MessageFilter } from '../api'
import './Pages.css'

function Messages() {
  const [messages, setMessages] = useState<Message[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [channelFilter, setChannelFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [fromDate, setFromDate] = useState(() => {
    const d = new Date()
    d.setDate(d.getDate() - 7)
    return d.toISOString().slice(0, 16)
  })
  const [toDate, setToDate] = useState(() => {
    return new Date().toISOString().slice(0, 16)
  })

  useEffect(() => {
    loadMessages()
  }, [page, channelFilter, statusFilter, fromDate, toDate])

  async function loadMessages() {
    setLoading(true)
    setError('')
    try {
      const filter: MessageFilter = {
        page,
        page_size: pageSize,
        from: new Date(fromDate).toISOString(),
        to: new Date(toDate).toISOString(),
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
    setFromDate('')
    setToDate('')
    setPage(1)
  }

  return (
    <div className="page">
      <h2 className="page-title">推送历史</h2>
      <p className="page-description">查看所有推送消息记录</p>

      <div className="filter-bar">
        <input
          type="text"
          className="text-input"
          placeholder="频道筛选"
          value={channelFilter}
          onChange={(e) => { setChannelFilter(e.target.value); setPage(1) }}
          aria-label="频道筛选"
        />
        <select
          className="select-input"
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1) }}
          aria-label="状态筛选"
        >
          <option value="">全部状态</option>
          <option value="success">成功</option>
          <option value="failed">失败</option>
          <option value="pending">待推送</option>
        </select>
        <input
          type="datetime-local"
          className="text-input"
          value={fromDate}
          onChange={(e) => { setFromDate(e.target.value); setPage(1) }}
          aria-label="开始时间"
        />
        <input
          type="datetime-local"
          className="text-input"
          value={toDate}
          onChange={(e) => { setToDate(e.target.value); setPage(1) }}
          aria-label="结束时间"
        />
        <button className="btn btn--secondary" onClick={handleReset}>重置</button>
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
