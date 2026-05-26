import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import ApiKeyGuard from './components/ApiKeyGuard'
import Dashboard from './pages/Dashboard'
import Messages from './pages/Messages'
import Reminders from './pages/Reminders'
import Push from './pages/Push'
import Devices from './pages/Devices'
import Channels from './pages/Channels'
import Settings from './pages/Settings'

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route path="settings" element={<Settings />} />
        <Route index element={<ApiKeyGuard><Dashboard /></ApiKeyGuard>} />
        <Route path="messages" element={<ApiKeyGuard><Messages /></ApiKeyGuard>} />
        <Route path="reminders" element={<ApiKeyGuard><Reminders /></ApiKeyGuard>} />
        <Route path="push" element={<ApiKeyGuard><Push /></ApiKeyGuard>} />
        <Route path="devices" element={<ApiKeyGuard><Devices /></ApiKeyGuard>} />
        <Route path="channels" element={<ApiKeyGuard><Channels /></ApiKeyGuard>} />
      </Route>
    </Routes>
  )
}

export default App
