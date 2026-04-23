import { BrowserRouter, Route, Routes } from 'react-router-dom'
import Top from './pages/Top'
import Room from './pages/Room'
import Footer from './components/Footer'
import './App.css'

// ThemeToggle is embedded per-page (inside Top's container and Room's header)
// so it doesn't collide with the room's top-right action buttons.
export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Top />} />
        <Route path="/r/:code" element={<Room />} />
      </Routes>
      <Footer />
    </BrowserRouter>
  )
}
