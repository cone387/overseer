import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import AuthGuard from './components/AuthGuard'
import Dashboard from './pages/Dashboard'
import Messages from './pages/Messages'
import Reminders from './pages/Reminders'
import Push from './pages/Push'
import Devices from './pages/Devices'
import Channels from './pages/Channels'
import ApiKeys from './pages/ApiKeys'
import Settings from './pages/Settings'
import DesktopLink from './pages/DesktopLink'

function App() {
  return (
    <AuthGuard>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="messages" element={<Messages />} />
          <Route path="reminders" element={<Reminders />} />
          <Route path="push" element={<Push />} />
          <Route path="devices" element={<Devices />} />
          <Route path="channels" element={<Channels />} />
          <Route path="api-keys" element={<ApiKeys />} />
          <Route path="settings" element={<Settings />} />
        </Route>
        <Route path="/desktop-link" element={<DesktopLink />} />
      </Routes>
    </AuthGuard>
  )
}

export default App
