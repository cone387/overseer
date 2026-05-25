import { useEffect, useState } from 'react'
import { fetchStats, ChannelStat } from '../api'
import { useWebSocket, WsEvent } from '../hooks/useWebSocket'
import './Pages.css'

function getTimeRange(hours: number): { from: string; to: string } {
  const to = new Date()
  const from = new Date(to.getTime() - hours * 60 * 60 * 1000)
  return {
    from: from.toISOString(),
    to: to.toISOString(),
  }
}

function Dashboard() {
  const [stats, setStats] = useState<ChannelStat[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [timeRange, setTimeRange] = useState(24)
  const [recentEvents, setRecentEvents] = useState<WsEvent[]>([])

  const { connected, events } = useWebSocket({
    onEvent: (event) => {
      setRecentEvents((prev) => [event, ...prev].slice(0, 20))
    },
  })

  useEffect(() => {
    setRecentEvents(events)
  }, [events])

  useEffect(() => {
    loadStats()
  }, [timeRange])

  async function loadStats() {
    setLoading(true)
    setError('')
    try {
      const { from, to } = getTimeRange(timeRange)
      const res = await fetchStats(from, to)
      setStats(res.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载统计数据失败')
    } finally {
      setLoading(false)
    }
  }

  const totalPushes = stats.reduce((sum, s) => sum + s.total, 0)
  const totalSuccess = stats.reduce((sum, s) => sum + s.success, 0)
  const totalFailed = stats.reduce((sum, s) => sum + s.failed, 0)
  const successRate = totalPushes > 0 ? ((totalSuccess / totalPushes) * 100).toFixed(1) : '--'

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h2 className="page-title">仪表盘</h2>
          <p className="page-description">推送统计概览</p>
        </div>
        <div className="header-actions">
          <span className={`ws-status ${connected ? 'ws-connected' : 'ws-disconnected'}`}>
            {connected ? '● 已连接' : '○ 未连接'}
          </span>
          <select
            className="select-input"
            value={timeRange}
            onChange={(e) => setTimeRange(Number(e.target.value))}
            aria-label="时间范围"
          >
            <option value={1}>最近 1 小时</option>
            <option value={6}>最近 6 小时</option>
            <option value={24}>最近 24 小时</option>
            <option value={168}>最近 7 天</option>
          </select>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-value">{loading ? '--' : totalPushes}</div>
          <div className="stat-label">总推送数</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{loading ? '--' : totalSuccess}</div>
          <div className="stat-label">成功</div>
        </div>
        <div className="stat-card">
          <div className="stat-value stat-value--danger">{loading ? '--' : totalFailed}</div>
          <div className="stat-label">失败</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{loading ? '--' : `${successRate}%`}</div>
          <div className="stat-label">成功率</div>
        </div>
      </div>

      {stats.length > 0 && (
        <div className="section">
          <h3 className="section-title">频道统计</h3>
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>频道</th>
                  <th>总数</th>
                  <th>成功</th>
                  <th>失败</th>
                  <th>占比</th>
                </tr>
              </thead>
              <tbody>
                {stats.map((s) => (
                  <tr key={s.channel}>
                    <td><span className="channel-badge">{s.channel}</span></td>
                    <td>{s.total}</td>
                    <td>{s.success}</td>
                    <td>{s.failed}</td>
                    <td>{s.percentage.toFixed(1)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <div className="section">
        <h3 className="section-title">实时事件</h3>
        {recentEvents.length === 0 ? (
          <div className="placeholder-content">
            <p>等待推送事件...</p>
          </div>
        ) : (
          <div className="event-list">
            {recentEvents.map((event, i) => (
              <div key={i} className="event-item">
                <span className="event-type">{event.type}</span>
                <span className="event-payload">
                  {(event.payload as Record<string, unknown>).title as string || JSON.stringify(event.payload)}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export default Dashboard
