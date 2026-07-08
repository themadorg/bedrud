const BASE_URL = (import.meta.env['VITE_API_URL'] as string | undefined) ?? ''

export function resolveAvatarUrl(url?: string | null): string | undefined {
  if (!url?.trim()) return undefined
  const trimmed = url.trim()
  if (trimmed.startsWith('http://') || trimmed.startsWith('https://') || trimmed.startsWith('data:')) {
    return trimmed
  }
  return `${BASE_URL}${trimmed.startsWith('/') ? trimmed : `/${trimmed}`}`
}
