import { jwtDecode } from 'jwt-decode'

interface BedrudJwt {
  userId?: string
  provider?: string
  accesses?: string[]
}

export function decodeBedrudJwt(token: string | undefined | null): {
  userId: string
  provider: string
  accesses: string[]
} {
  if (!token) return { userId: '', provider: '', accesses: [] }
  try {
    const payload = jwtDecode<BedrudJwt>(token)
    return {
      userId: payload.userId ?? '',
      provider: payload.provider ?? '',
      accesses: payload.accesses ?? [],
    }
  } catch {
    return { userId: '', provider: '', accesses: [] }
  }
}

export function isGuestToken(token: string | undefined | null): boolean {
  const { provider, accesses } = decodeBedrudJwt(token)
  return provider === 'guest' || accesses.includes('guest')
}
