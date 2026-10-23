import { useEffect, useState } from 'react'
import { api } from './api'

function App() {
  const [version, setVersion] = useState('…')
  const [error, setError] = useState('')

  useEffect(() => {
    api.version()
      .then(setVersion)
      .catch((e) => setError(String(e)))
  }, [])

  return (
    <main className="boot" style={{ paddingTop: 52 }}>
      <h1>Hermit</h1>
      <p className="muted">
        {error ? `后端连接失败：${error}` : `核心已就绪 · v${version}`}
      </p>
    </main>
  )
}

export default App
