export async function createRoom(): Promise<{ code: string }> {
  const r = await fetch('/api/rooms', { method: 'POST' })
  if (!r.ok) throw new Error('create failed')
  return r.json()
}

export async function checkRoom(code: string): Promise<{ exists: boolean }> {
  const r = await fetch(`/api/rooms/${encodeURIComponent(code)}`)
  if (!r.ok) throw new Error('check failed')
  return r.json()
}

export async function joinRoom(
  code: string,
  name: string,
): Promise<{ participantId: string; roomId: string; name: string }> {
  const r = await fetch(`/api/rooms/${encodeURIComponent(code)}/join`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ name }),
  })
  if (!r.ok) throw new Error(`join failed: ${r.status}`)
  return r.json()
}
