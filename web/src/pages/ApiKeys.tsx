import { useState, useEffect } from 'react'
import {
  listAPIKeys,
  createAPIKey,
  deleteAPIKey,
  ApiError,
  type APIKey,
} from '../api'
import './Pages.css'

function ApiKeys() {
  const [keys, setKeys] = useState<APIKey[]>([])
  const [newKeyName, setNewKeyName] = useState('')
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<APIKey | null>(null)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    loadKeys()
  }, [])

  async function loadKeys() {
    try {
      const list = await listAPIKeys()
      setKeys(list)
    } catch {
      // ignore
    }
  }

  async function handleCreateKey() {
    if (!newKeyName.trim()) return
    setError('')
    try {
      const key = await createAPIKey(newKeyName.trim())
      setNewlyCreatedKey(key)
      setNewKeyName('')
      loadKeys()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '创建失败')
    }
  }

  async function handleDeleteKey(id: string) {
    try {
      await deleteAPIKey(id)
      setKeys((prev) => prev.filter((k) => k.id !== id))
      if (newlyCreatedKey?.id === id) {
        setNewlyCreatedKey(null)
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '删除失败')
    }
  }

  async function handleCopyKey(key: string) {
    try {
      await navigator.clipboard.writeText(key)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // fallback
    }
  }

  return (
    <div className="page">
      <h2 className="page-title">API Key 管理</h2>
      <p className="page-description">API Key 用于外部程序调用推送接口。生成后完整密钥仅显示一次，请妥善保存。</p>

      <div className="form-card">
        {error && <div className="error-banner">{error}</div>}

        {/* New key creation */}
        <div className="form-grid" style={{ marginBottom: 16 }}>
          <div className="form-row">
            <div className="form-group" style={{ flex: 1 }}>
              <input
                type="text"
                className="text-input"
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                placeholder="Key 名称（如：服务器A）"
                onKeyDown={(e) => { if (e.key === 'Enter') handleCreateKey() }}
              />
            </div>
            <button
              className="btn btn--primary"
              onClick={handleCreateKey}
              disabled={!newKeyName.trim()}
            >
              生成新 Key
            </button>
          </div>
        </div>

        {/* Newly created key display */}
        {newlyCreatedKey?.key && (
          <div className="result-card result-card--success" style={{ marginBottom: 16 }}>
            <div className="result-content" style={{ flex: 1 }}>
              <div className="result-message">新 Key 已生成：{newlyCreatedKey.name}</div>
              <div className="settings-new-key-display">
                <code className="settings-key-value">{newlyCreatedKey.key}</code>
                <button
                  className="btn btn--secondary btn--sm"
                  onClick={() => handleCopyKey(newlyCreatedKey.key!)}
                >
                  {copied ? '✓ 已复制' : '复制'}
                </button>
              </div>
              <small className="form-hint">⚠️ 此密钥仅显示一次，关闭后无法再次查看</small>
            </div>
          </div>
        )}

        {/* Key list */}
        {keys.length > 0 ? (
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>前缀</th>
                  <th>创建时间</th>
                  <th>最后使用</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => (
                  <tr key={k.id}>
                    <td>{k.name}</td>
                    <td><code>{k.prefix}••••</code></td>
                    <td className="td-nowrap">{formatDate(k.created_at)}</td>
                    <td className="td-nowrap">{k.last_used ? formatDate(k.last_used) : '从未使用'}</td>
                    <td>
                      <button
                        className="btn btn--danger btn--sm"
                        onClick={() => handleDeleteKey(k.id)}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="table-empty">暂无 API Key</div>
        )}
      </div>
    </div>
  )
}

function formatDate(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return iso
  }
}

export default ApiKeys
