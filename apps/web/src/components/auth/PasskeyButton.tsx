import { Link } from '@tanstack/react-router'
import { Fingerprint } from 'lucide-react'
import { useState } from 'react'
import { api } from '#/lib/api'
import {
  base64ToBuffer,
  bufferToBase64,
  type PublicKeyCredentialCreationOptionsJSON,
  type PublicKeyCredentialRequestOptionsJSON,
} from '#/lib/webauthn'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

interface AuthResponse {
  user: { id: string; email: string; name: string; provider: string; accesses: string[] | null; avatarUrl?: string }
  tokens: { accessToken: string; refreshToken: string }
}

interface VerificationRequiredResponse {
  requiresVerification: true
  email: string
  message: string
}

type SignupResponse = AuthResponse | VerificationRequiredResponse

export async function loginWithPasskey(email?: string): Promise<AuthResponse> {
  const trimmed = email?.trim()
  const opts = await api.post<PublicKeyCredentialRequestOptionsJSON>(
    '/api/auth/passkey/login/begin',
    trimmed ? { email: trimmed } : {},
  )
  if (trimmed && (!opts.allowCredentials || opts.allowCredentials.length === 0)) {
    throw new Error('No passkey found for this email. Sign in with password or register a passkey first.')
  }

  const publicKey: PublicKeyCredentialRequestOptions = {
    challenge: base64ToBuffer(opts.challenge as unknown as string),
    timeout: opts.timeout,
    rpId: opts.rpId,
    userVerification: opts.userVerification ?? 'preferred',
  }
  if (opts.allowCredentials?.length) {
    publicKey.allowCredentials = opts.allowCredentials.map((c) => ({
      id: base64ToBuffer(c.id),
      type: 'public-key' as const,
    }))
  }

  const cred = (await navigator.credentials.get({ publicKey })) as PublicKeyCredential | null
  if (!cred) {
    throw new Error('Passkey authentication was cancelled')
  }

  const assertion = cred.response as AuthenticatorAssertionResponse
  return api.post<AuthResponse>('/api/auth/passkey/login/finish', {
    credentialId: bufferToBase64(cred.rawId),
    clientDataJSON: bufferToBase64(assertion.clientDataJSON),
    authenticatorData: bufferToBase64(assertion.authenticatorData),
    signature: bufferToBase64(assertion.signature),
  })
}

export async function signupWithPasskey(name: string, email: string): Promise<SignupResponse> {
  const opts = await api.post<PublicKeyCredentialCreationOptionsJSON>('/api/auth/passkey/signup/begin', {
    name,
    email,
  })
  const optsRaw = opts as unknown as {
    rp: PublicKeyCredentialRpEntity
    user: { id: string; name: string; displayName: string }
    challenge: string
    pubKeyCredParams: PublicKeyCredentialParameters[]
    timeout?: number
    attestation?: AttestationConveyancePreference
    authenticatorSelection?: AuthenticatorSelectionCriteria
  }
  const cred = (await navigator.credentials.create({
    publicKey: {
      rp: optsRaw.rp,
      user: { id: base64ToBuffer(optsRaw.user.id), name: optsRaw.user.name, displayName: optsRaw.user.displayName },
      challenge: base64ToBuffer(optsRaw.challenge),
      pubKeyCredParams: optsRaw.pubKeyCredParams,
      timeout: optsRaw.timeout,
      attestation: optsRaw.attestation,
      authenticatorSelection: optsRaw.authenticatorSelection,
    },
  })) as PublicKeyCredential | null
  if (!cred) {
    throw new Error('Passkey setup was cancelled')
  }

  const att = cred.response as AuthenticatorAttestationResponse
  return api.post<SignupResponse>('/api/auth/passkey/signup/finish', {
    clientDataJSON: bufferToBase64(att.clientDataJSON),
    attestationObject: bufferToBase64(att.attestationObject),
  })
}

interface Props {
  onSuccess: (res: AuthResponse) => void
  mode?: 'login' | 'signup' | 'both'
  compact?: boolean
  className?: string
}

export function PasskeyButton({ onSuccess, mode = 'both', compact = false, className }: Props) {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [verificationEmail, setVerificationEmail] = useState<string | null>(null)

  async function handleLogin() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await loginWithPasskey()
      onSuccess(res)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Passkey login failed')
    } finally {
      setIsLoading(false)
    }
  }

  async function handleSignup(name: string, email: string) {
    setIsLoading(true)
    setError(null)
    try {
      const res = await signupWithPasskey(name, email)
      if ('requiresVerification' in res && res.requiresVerification) {
        setVerificationEmail(res.email)
        return
      }
      onSuccess(res as AuthResponse)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Passkey signup failed')
    } finally {
      setIsLoading(false)
    }
  }

  if (verificationEmail) {
    return (
      <div className={cn('space-y-3', className)}>
        <div className="border border-primary/20 bg-primary/5 px-4 py-3 text-sm">
          <p className="font-medium">Verify your email</p>
          <p className="mt-1 text-muted-foreground">
            We sent a verification email to <span className="font-medium text-foreground">{verificationEmail}</span>.
          </p>
        </div>
        <p className="text-xs text-muted-foreground">
          Didn't receive it?{' '}
          <Link
            to="/auth/login"
            search={{ redirect: undefined }}
            className="underline underline-offset-4 hover:no-underline"
          >
            Sign in to resend
          </Link>
        </p>
      </div>
    )
  }

  const errorBlock = error ? (
    <div className="rounded-md bg-destructive/15 px-3 py-2 text-sm text-destructive">{error}</div>
  ) : null

  if (mode === 'login') {
    return (
      <div className={cn('space-y-3', className)}>
        {errorBlock}
        <Button onClick={handleLogin} className="w-full" disabled={isLoading}>
          <Fingerprint className="me-2 h-4 w-4" />
          {isLoading ? 'Authenticating…' : 'Sign in with Passkey'}
        </Button>
      </div>
    )
  }

  if (mode === 'signup') {
    return (
      <div className={cn('space-y-3', className)}>
        {errorBlock}
        <PasskeySignupForm onSignup={handleSignup} isLoading={isLoading} />
      </div>
    )
  }

  if (compact) {
    return (
      <div className={cn('space-y-4', className)}>
        {errorBlock}
        <Tabs defaultValue="login">
          <TabsList className="w-full">
            <TabsTrigger value="login" className="flex-1">
              Login
            </TabsTrigger>
            <TabsTrigger value="signup" className="flex-1">
              Sign up
            </TabsTrigger>
          </TabsList>
          <TabsContent value="login" className="pt-4">
            <Button onClick={handleLogin} className="w-full" disabled={isLoading}>
              <Fingerprint className="me-2 h-4 w-4" />
              {isLoading ? 'Authenticating…' : 'Sign in with Passkey'}
            </Button>
          </TabsContent>
          <TabsContent value="signup" className="pt-4">
            <PasskeySignupForm onSignup={handleSignup} isLoading={isLoading} />
          </TabsContent>
        </Tabs>
      </div>
    )
  }

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Fingerprint className="h-5 w-5" />
          Passkey
        </CardTitle>
        <CardDescription>Sign in or sign up using a biometric authenticator</CardDescription>
      </CardHeader>
      <CardContent>
        {errorBlock ? <div className="mb-4">{errorBlock}</div> : null}
        <Tabs defaultValue="login">
          <TabsList className="w-full">
            <TabsTrigger value="login" className="flex-1">
              Login
            </TabsTrigger>
            <TabsTrigger value="signup" className="flex-1">
              Sign up
            </TabsTrigger>
          </TabsList>
          <TabsContent value="login" className="pt-4">
            <Button onClick={handleLogin} className="w-full" disabled={isLoading}>
              <Fingerprint className="me-2 h-4 w-4" />
              {isLoading ? 'Authenticating…' : 'Sign in with Passkey'}
            </Button>
          </TabsContent>
          <TabsContent value="signup" className="pt-4">
            <PasskeySignupForm onSignup={handleSignup} isLoading={isLoading} />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}

function PasskeySignupForm({
  onSignup,
  isLoading,
}: {
  onSignup: (name: string, email: string) => Promise<void>
  isLoading: boolean
}) {
  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget as HTMLFormElement)
    await onSignup(fd.get('name') as string, fd.get('username') as string)
  }

  return (
    <form method="post" action="#" onSubmit={handleSubmit} className="space-y-3" autoComplete="on">
      <div className="space-y-1">
        <Label htmlFor="pk-name">Name</Label>
        <Input id="pk-name" name="name" placeholder="Your name" required autoComplete="name" />
      </div>
      <div className="space-y-1">
        <Label htmlFor="pk-username">Email</Label>
        <Input
          id="pk-username"
          name="username"
          type="email"
          inputMode="email"
          placeholder="you@example.com"
          required
          autoComplete="username webauthn"
        />
      </div>
      <Button type="submit" className="w-full" disabled={isLoading}>
        <Fingerprint className="me-2 h-4 w-4" />
        {isLoading ? 'Setting up…' : 'Create account with Passkey'}
      </Button>
    </form>
  )
}
