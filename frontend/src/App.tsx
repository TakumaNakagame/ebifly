import { BrowserRouter, Route, Routes } from 'react-router-dom'
import Top from './pages/Top'
import Room from './pages/Room'
import './App.css'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Top />} />
        <Route path="/r/:code" element={<Room />} />
      </Routes>
    </BrowserRouter>
  )
}
