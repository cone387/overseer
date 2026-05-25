import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Messages from './pages/Messages'
import Reminders from './pages/Reminders'
import Push from './pages/Push'

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Dashboard />} />
        <Route path="messages" element={<Messages />} />
        <Route path="reminders" element={<Reminders />} />
        <Route path="push" element={<Push />} />
      </Route>
    </Routes>
  )
}

export default App
