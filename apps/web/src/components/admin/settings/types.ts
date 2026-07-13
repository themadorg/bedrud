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
  chatUploadMaxDimension: number
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

  // Experimental WebXDC gallery (admin-configurable; requires webxdc.enabled + domain in config.yaml)
  webxdcGalleryEnabled: boolean
  /** Remote catalog JSON/base URL (e.g. store API or self-hosted mirror). */
  webxdcGalleryRemoteCatalogUrl: string
  /** local | remote | both | semi-remote */
  webxdcGallerySource: string
  /** If true, server may download .xdc from catalog entry URLs. */
  webxdcGalleryAllowRemoteDownload: boolean
  /** Admin-uploaded instance catalog packages (in addition to remote/semi-remote). */
  webxdcGalleryInstanceCatalogEnabled: boolean
  /** Max .xdc archive size in MiB (default 10). */
  webxdcMaxArchiveMB: number
  /** Max total uncompressed zip size in MiB (default 30). */
  webxdcMaxUncompressedMB: number
  /** Max single file inside .xdc in MiB (default 5) — usual "entry too large" limit. */
  webxdcMaxSingleFileMB: number
  /** Max files inside a .xdc package (default 500). */
  webxdcMaxEntries: number

  /** Instance allows RNNoise WASM (off by default; not downloaded until enabled + selected). */
  rnnoiseEnabled: boolean
  /** Instance allows users to select Krisp (off by default; requires operator licensing). */
  krispEnabled: boolean

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
