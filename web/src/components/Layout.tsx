import { NavLink, Outlet } from 'react-router-dom'
import './Layout.css'

const navItems = [
  { path: '/', label: '仪表盘', icon: '📊' },
  { path: '/messages', label: '推送历史', icon: '📨' },
  { path: '/reminders', label: '提醒管理', icon: '⏰' },
  { path: '/push', label: '发送推送', icon: '🚀' },
  { path: '/devices', label: '设备管理', icon: '📱' },
]

function Layout() {
  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-header">
          <h1 className="sidebar-title">Overseer</h1>
          <p className="sidebar-subtitle">推送管理系统</p>
        </div>
        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              end={item.path === '/'}
              className={({ isActive }) =>
                `nav-item ${isActive ? 'nav-item--active' : ''}`
              }
            >
              <span className="nav-icon">{item.icon}</span>
              <span className="nav-label">{item.label}</span>
            </NavLink>
          ))}
        </nav>
      </aside>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  )
}

export default Layout
