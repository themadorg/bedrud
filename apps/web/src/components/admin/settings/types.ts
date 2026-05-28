export interface SystemSettings {
  id: number
  registrationEnabled: boolean
  tokenRegistrationOnly: boolean
  guestLoginEnabled: boolean
  passkeysEnabled: boolean
  googleClientId: string
  googleClientSecret: string
  googleRedirectUrl: string
  githubClientId: string
  githubClientSecret: string
  githubRedirectUrl: string
  twitterClientId: string
  twitterClientSecret: string
  twitterRedirectUrl: string
  jwtSecret: string
  tokenDuration: number
  sessionSecret: string
  frontendUrl: string
  serverPort: string
  serverHost: string
  serverDomain: string
  serverEnableTls: boolean
  serverCertFile: string
  serverKeyFile: string
  serverUseAcme: boolean
  serverEmail: string
  behindProxy: boolean
  serverName: string
  livekitHost: string
  livekitApiKey: string
  livekitApiSecret: string
  livekitExternal: boolean
  corsAllowedOrigins: string
  corsAllowedHeaders: string
  corsAllowedMethods: string
  corsAllowCredentials: boolean
  corsMaxAge: number
  chatUploadBackend: string
  chatUploadMaxBytes: number
  chatUploadInlineMax: number
  chatUploadDiskDir: string
  chatUploadS3Endpoint: string
  chatUploadS3Bucket: string
  chatUploadS3Region: string
  chatUploadS3AccessKey: string
  chatUploadS3SecretKey: string
  chatUploadS3PublicUrl: string
  logLevel: string
  maxParticipantsLimit: number
  maxRoomsPerUser: number
  maxUploadBytesPerUser: number
  globalDiskThresholdBytes: number
  chatMaxMessageCount: number
  chatMessageTTLHours: number

  // TODO oncoming feature
  // Recordings
  recordingsEnabled: boolean
  // TODO oncoming feature
  recordingMaxDurationMins: number
  // TODO oncoming feature
  recordingMaxFileSizeMB: number

  // Email branding
  emailInstanceName: string
  emailSupportEmail: string
  emailInstanceUrl: string
  emailHeaderBg: string
  emailButtonBg: string

  // Per-template subject overrides
  emailSubjectVerify: string
  emailSubjectWelcome: string
  emailSubjectReset: string
  emailSubjectChanged: string
  emailSubjectInvite: string

  // Per-template preheader text
  emailPreheaderVerify: string
  emailPreheaderWelcome: string
  emailPreheaderReset: string
  emailPreheaderChanged: string
  emailPreheaderInvite: string

  // SMTP settings
  emailSmtpHost: string
  emailSmtpPort: number
  emailUsername: string
  emailPassword: string
  emailFromAddress: string
  emailFromName: string
  emailTlsSkipVerify: boolean
  emailSmtpsMode: boolean

  updatedAt: string
}

export interface InviteToken {
  id: string
  token: string
  email: string
  createdBy: string
  expiresAt: string
  usedAt: string | null
  usedBy: string
  createdAt: string
}

export type RegMode = 'open' | 'invite' | 'closed'
