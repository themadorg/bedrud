#!/usr/bin/env node
/**
 * Capture PNGs of every Bedrud web section via Puppeteer.
 * Long pages are sliced as name_1.png, name_2.png, … (viewport scroll).
 *
 * Usage (from repo root or this folder):
 *   node tools/screenshots/screenshot.js
 *   node tools/screenshots/screenshot.js --base-url http://127.0.0.1:7071 --no-start
 *
 * The app is captured as it ships: the Go binary serving the embedded bundle from
 * server/frontend, with the API on the same origin. `vite dev` is not an
 * equivalent target — it adds a TanStack Start SSR pass whose dehydrated route
 * loaders leave every dashboard page rendered signed out (#129).
 *
 * Env:
 *   BEDRUD_BASE_URL              default http://127.0.0.1:7071
 *   BEDRUD_API_URL               default http://127.0.0.1:7071 (login + start probe)
 *   BEDRUD_SCREENSHOT_EMAIL      account used for dashboard/admin pages
 *   BEDRUD_SCREENSHOT_PASSWORD
 *   BEDRUD_SCREENSHOT_OUT        output directory (default tools/screenshots/output)
 *   BEDRUD_SCREENSHOT_NO_START   set to 1 to skip spawning the app
 *   BEDRUD_SESSION_TIMEOUT       ms to wait for an authenticated page to show the
 *                                signed-in account before failing (default 30000)
 */

import { spawn } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import puppeteer from 'puppeteer'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(__dirname, '../..')

const args = parseArgs(process.argv.slice(2))

const BASE_URL = trimSlash(args['base-url'] || process.env.BEDRUD_BASE_URL || 'http://127.0.0.1:7071')
const API_URL = trimSlash(args['api-url'] || process.env.BEDRUD_API_URL || 'http://127.0.0.1:7071')
const EMAIL = args.email || process.env.BEDRUD_SCREENSHOT_EMAIL || ''
const PASSWORD = args.password || process.env.BEDRUD_SCREENSHOT_PASSWORD || ''
const OUT_DIR = path.resolve(
  args.out || process.env.BEDRUD_SCREENSHOT_OUT || path.join(__dirname, 'output'),
)
const SHOULD_START =
  !args['no-start'] && process.env.BEDRUD_SCREENSHOT_NO_START !== '1'
const HEADLESS = args.headful ? false : true
const TIMEOUT_MS = Number(args.timeout || 30_000)
const SESSION_TIMEOUT_MS = Number(args['session-timeout'] || process.env.BEDRUD_SESSION_TIMEOUT || 30_000)
const LIVEKIT_URL = trimSlash(args['livekit-url'] || process.env.BEDRUD_LIVEKIT_URL || 'http://127.0.0.1:7072')
/** Iranian names + Koboyo face icons as profile pictures. */
const HOST_PERSON = {
  name: 'Shahram Farhadi',
  email: EMAIL,
  icon: 'athlete',
}
const PEOPLE = [
  { name: 'Kiarash Rostami', email: 'kiarash.shot@bedrud.local', icon: 'bored', line: 'Hey — can everyone hear me?' },
  { name: 'Parsa Kaviani', email: 'parsa.shot@bedrud.local', icon: 'cyborg', line: 'Yes, loud and clear.' },
  { name: 'Shirin Golestan', email: 'shirin.shot@bedrud.local', icon: 'architect', line: 'Ready when you are.' },
  { name: 'Arash Mehran', email: 'arash.shot@bedrud.local', icon: 'cheeky', line: 'Starting in two minutes.' },
  { name: 'Azadeh Sepehri', email: 'azadeh.shot@bedrud.local', icon: 'lawyer-laptop', line: 'I am here too.' },
]

const VIEWPORTS = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'mobile', width: 390, height: 844, isMobile: true, hasTouch: true },
]

const THEMES = ['light', 'dark']

/** Public pages — no login. */
const PUBLIC_PAGES = [
  { id: 'landing', path: '/' },
  { id: 'auth-guest', path: '/auth' },
  { id: 'auth-login', path: '/auth/login' },
  { id: 'auth-register', path: '/auth/register' },
  { id: 'auth-verify', path: '/auth/verify' },
  { id: 'auth-forgot-password', path: '/auth/forgot-password' },
  { id: 'auth-reset-password', path: '/auth/reset-password?token=screenshot-placeholder' },
  { id: 'auth-callback', path: '/auth/callback' },
  { id: 'not-found', path: '/this-page-does-not-exist' },
]

/** Requires a logged-in user. */
const AUTHED_PAGES = [
  { id: 'dashboard', path: '/dashboard' },
  { id: 'dashboard-archived', path: '/dashboard/archived/screenshot-room' },
  { id: 'settings-profile', path: '/dashboard/settings' },
  { id: 'settings-security', path: '/dashboard/settings/security' },
  { id: 'settings-audio', path: '/dashboard/settings/audio' },
  { id: 'settings-video', path: '/dashboard/settings/video' },
]

/** Requires admin / superadmin. */
const ADMIN_PAGES = [
  { id: 'admin-overview', path: '/dashboard/admin' },
  { id: 'admin-queue', path: '/dashboard/admin/queue' },
  { id: 'admin-rooms', path: '/dashboard/admin/rooms' },
  { id: 'admin-rooms-detail', path: '/dashboard/admin/rooms/screenshot-room' },
  { id: 'admin-rooms-events', path: '/dashboard/admin/rooms/events' },
  { id: 'admin-users', path: '/dashboard/admin/users' },
  { id: 'admin-users-detail', path: '/dashboard/admin/users/screenshot-user' },
  { id: 'admin-users-recent-signups', path: '/dashboard/admin/users/recent-signups' },
  { id: 'admin-recordings', path: '/dashboard/admin/recordings' },
  { id: 'admin-settings-general', path: '/dashboard/admin/settings' },
  { id: 'admin-settings-auth', path: '/dashboard/admin/settings?tab=auth' },
  { id: 'admin-settings-livekit', path: '/dashboard/admin/settings?tab=livekit' },
  { id: 'admin-settings-server', path: '/dashboard/admin/settings?tab=server' },
  { id: 'admin-settings-email', path: '/dashboard/admin/settings?tab=email' },
  { id: 'admin-settings-cors', path: '/dashboard/admin/settings?tab=cors' },
  { id: 'admin-settings-chat', path: '/dashboard/admin/settings?tab=chat' },
  { id: 'admin-settings-webxdc', path: '/dashboard/admin/settings?tab=webxdc' },
  { id: 'admin-settings-logging', path: '/dashboard/admin/settings?tab=logging' },
  { id: 'admin-settings-webhooks', path: '/dashboard/admin/settings?tab=webhooks' },
  { id: 'admin-settings-audio', path: '/dashboard/admin/settings/audio' },
]

const children = []

/** Slugs of session-bearing pages that rendered their signed-out shell. */
const sessionFailures = []

/** A page whose capture is only meaningful once the session has taken. */
const withSession = (pageDef) => ({ ...pageDef, requiresSession: true })

process.on('SIGINT', () => shutdown(130))
process.on('SIGTERM', () => shutdown(143))

main().catch((err) => {
  console.error(err)
  shutdown(1)
})

async function main() {
  await mkdir(OUT_DIR, { recursive: true })

  if (SHOULD_START) {
    await startStack()
  }

  await waitForHttp(`${BASE_URL}/`, TIMEOUT_MS)
  console.log(`web ready: ${BASE_URL}`)

  const apiUp = await ping(`${API_URL}/api/health`).catch(() =>
    ping(`${API_URL}/api/auth/settings`).catch(() => false),
  )
  if (!apiUp) {
    console.warn(`API not reachable at ${API_URL} — authenticated pages will be skipped unless you pass --email after starting the server`)
  }

  const session = apiUp && EMAIL && PASSWORD ? await login(API_URL, EMAIL, PASSWORD) : null
  if (EMAIL && !session) {
    console.warn('login failed or skipped — capturing public pages only')
  } else if (session) {
    console.log(`logged in as ${EMAIL}`)
    await enableExperimentalPrefs(API_URL, session.accessToken)
  }

  const launchOpts = {
    headless: HEADLESS,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--font-render-hinting=none',
      '--disable-dev-shm-usage',
      '--use-fake-ui-for-media-stream',
      '--use-fake-device-for-media-stream',
      '--autoplay-policy=no-user-gesture-required',
      '--enable-usermedia-screen-capturing',
      '--auto-select-desktop-capture-source=Entire screen',
    ],
    defaultViewport: null,
  }
  if (process.env.PUPPETEER_EXECUTABLE_PATH) {
    launchOpts.executablePath = process.env.PUPPETEER_EXECUTABLE_PATH
  }

  const browser = await puppeteer.launch(launchOpts)

  const manifest = []
  const extras = []
  let roomName = ''

  try {
    if (session) {
      roomName = `shot-${Date.now().toString(36)}`
      const created = await createRoom(API_URL, session.accessToken, roomName)
      if (created) {
        const pngs = await rasterizeKoboyoIcons(browser)
        if (session) {
          await setupPersonProfile(API_URL, session.accessToken, HOST_PERSON.name, pngs[HOST_PERSON.icon])
        }
        console.log(`created public room ${roomName} — joining ${PEOPLE.length} extra people`)
        for (const person of PEOPLE) {
          extras.push(await joinAsPerson(browser, roomName, person, pngs[person.icon]))
        }
      } else {
        console.warn('could not create LiveKit room — meeting grid shots will be skipped')
        roomName = ''
      }
    }

    for (const viewport of VIEWPORTS) {
      for (const theme of THEMES) {
        const pages = [...PUBLIC_PAGES]
        if (session) {
          pages.push(...AUTHED_PAGES.map(withSession), ...ADMIN_PAGES.map(withSession))
        }
        if (roomName) {
          pages.push(
            { id: 'meeting-welcome', path: `/m/${roomName}`, kind: 'meeting-welcome' },
            { id: 'meeting-guest-join', path: `/m/${roomName}`, kind: 'meeting-guest-join' },
            { id: 'meeting-grid', path: `/m/${roomName}`, kind: 'meeting' },
            { id: 'meeting-chat', path: `/m/${roomName}`, kind: 'meeting-chat' },
            { id: 'meeting-participants', path: `/m/${roomName}`, kind: 'meeting-participants' },
            { id: 'meeting-info', path: `/m/${roomName}`, kind: 'meeting-info' },
            { id: 'meeting-more', path: `/m/${roomName}`, kind: 'meeting-more' },
            { id: 'meeting-settings', path: `/m/${roomName}`, kind: 'meeting-settings' },
          )
          // Stage features: desktop only (screen picker + dialogs are unusable on the mobile bar).
          if (viewport.name === 'desktop') {
            pages.push(
              { id: 'meeting-screenshare', path: `/m/${roomName}`, kind: 'meeting-screenshare' },
              { id: 'meeting-webxdc', path: `/m/${roomName}`, kind: 'meeting-webxdc' },
              { id: 'meeting-whiteboard', path: `/m/${roomName}`, kind: 'meeting-whiteboard' },
              { id: 'meeting-youtube', path: `/m/${roomName}`, kind: 'meeting-youtube' },
            )
          }
        } else {
          pages.push({ id: 'meeting-join', path: '/m/screenshot-room' })
        }

        for (const pageDef of pages) {
          const files = await capturePage(browser, {
            pageDef,
            viewport,
            theme,
            session,
            expectPeople: Boolean(pageDef.kind?.startsWith('meeting') && pageDef.kind !== 'meeting-guest-join' && roomName),
            skipAuth: pageDef.kind === 'meeting-guest-join',
            peopleCount: 1 + extras.filter((e) => e?.page).length,
            extras,
          })
          if (Array.isArray(files)) manifest.push(...files)
          else if (files) manifest.push(files)
        }
      }
    }
  } finally {
    for (const extra of extras) {
      await extra?.page?.close().catch(() => {})
      await extra?.context?.close().catch(() => {})
    }
    await browser.close()
  }

  const manifestPath = path.join(OUT_DIR, 'manifest.json')
  await writeFile(
    manifestPath,
    `${JSON.stringify({ capturedAt: new Date().toISOString(), baseUrl: BASE_URL, files: manifest }, null, 2)}\n`,
  )
  console.log(`wrote ${manifest.length} screenshots → ${OUT_DIR}`)

  // Fail loudly rather than publishing a gallery of signed-out shells. The
  // manifest is written first so the run's artifacts survive for inspection.
  if (sessionFailures.length) {
    console.error(`\n${sessionFailures.length} authenticated page(s) never signed in as ${EMAIL}:`)
    // "signed out:" prefixed, not bare — a bare slug is how a captured page is
    // logged, and the two lists are otherwise indistinguishable in CI output.
    for (const slug of sessionFailures) console.error(`  signed out: ${slug}`)
    console.error(`evidence: ${path.join(OUT_DIR, 'failures')}`)
    shutdown(1)
  }

  shutdown(0)
}

async function capturePage(browser, { pageDef, viewport, theme, session, expectPeople, peopleCount, skipAuth, extras }) {
  const context = await browser.createBrowserContext()
  await context.overridePermissions(BASE_URL, ['camera', 'microphone'])
  const page = await context.newPage()
  const slug = `${pageDef.id}__${theme}__${viewport.name}`

  try {
    await page.setViewport({
      width: viewport.width,
      height: viewport.height,
      deviceScaleFactor: 1,
      isMobile: Boolean(viewport.isMobile),
      hasTouch: Boolean(viewport.hasTouch),
    })
    await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }])

    const skipWelcome = Boolean(expectPeople) && pageDef.kind !== 'meeting-welcome'
    await page.evaluateOnNewDocument(
      ({ themeValue, sessionValue, skipWelcomeValue }) => {
        localStorage.setItem('theme', themeValue)
        localStorage.setItem(
          'experimental-preferences',
          JSON.stringify({
            state: {
              whiteboardEnabled: true,
              youtubeEnabled: true,
              webxdcEnabled: true,
              whiteboardDisclaimerAcknowledged: true,
            },
            version: 0,
          }),
        )
        if (skipWelcomeValue) {
          localStorage.setItem(
            'interface-preferences',
            JSON.stringify({ state: { showWelcomeScreen: false }, version: 0 }),
          )
        }
        if (sessionValue) {
          localStorage.setItem('auth_remember', '1')
          localStorage.setItem('auth_at', sessionValue.accessToken)
        }
      },
      {
        themeValue: theme,
        sessionValue: skipAuth ? null : session,
        skipWelcomeValue: skipWelcome,
      },
    )

    if (session && !skipAuth) {
      const origin = new URL(BASE_URL)
      await page.setCookie(
        {
          name: 'access_token',
          value: session.accessToken,
          domain: origin.hostname,
          path: '/',
          httpOnly: true,
          sameSite: 'Lax',
        },
        {
          name: 'refresh_token',
          value: session.refreshToken,
          domain: origin.hostname,
          path: '/',
          httpOnly: true,
          sameSite: 'Lax',
        },
      )
    }

    const url = `${BASE_URL}${pageDef.path}`
    // Vite HMR keeps a WS open; do not wait for networkidle.
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: TIMEOUT_MS })
    await settle(page)

    if (pageDef.kind === 'meeting-guest-join') {
      await page.waitForSelector('input[placeholder="Your name"]', { timeout: TIMEOUT_MS })
      await sleep(400)
    } else if (pageDef.kind === 'meeting-welcome') {
      await page.waitForSelector('button', { timeout: TIMEOUT_MS })
      await page.waitForFunction(
        () => [...document.querySelectorAll('button')].some((b) => b.textContent.trim() === 'Join meeting'),
        { timeout: TIMEOUT_MS },
      )
      await sleep(600)
    } else if (expectPeople) {
      await enterMeetingFromWelcome(page)
      await waitForPeople(page, Math.min(peopleCount || 2, 4))
      await applyMeetingChrome(page, pageDef.kind, extras)
      await sleep(700)
    }

    if (pageDef.requiresSession && session && !skipAuth) {
      await waitForSignedIn(page, slug)
    }

    const slices = await captureScrollSlices(page, slug, {
      // Meeting chrome is a fixed viewport — one frame is the whole UI.
      skipScroll: Boolean(pageDef.kind?.startsWith('meeting')),
    })
    for (const name of slices) console.log(`  ${name.replace(/\.png$/, '')}`)
    return slices.map((file, index) => ({
      id: pageDef.id,
      path: pageDef.path,
      theme,
      viewport: viewport.name,
      file,
      slice: index + 1,
      slices: slices.length,
    }))
  } catch (err) {
    console.error(`  FAIL ${slug}: ${err.message}`)
    return {
      id: pageDef.id,
      path: pageDef.path,
      theme,
      viewport: viewport.name,
      file: null,
      error: err.message,
    }
  } finally {
    await page.close().catch(() => {})
    await context.close().catch(() => {})
  }
}

/**
 * Viewport-sized shots. Short pages stay unsuffixed.
 * Pages taller than the window are saved as slug_1.png, slug_2.png, …
 */
async function captureScrollSlices(page, slug, { skipScroll } = {}) {
  if (skipScroll) {
    const dest = path.join(OUT_DIR, `${slug}.png`)
    await page.screenshot({ path: dest, fullPage: false })
    return [path.basename(dest)]
  }

  const metrics = await page.evaluate(() => {
    const doc = document.scrollingElement || document.documentElement
    const winH = window.innerHeight
    const docExtra = doc.scrollHeight - winH

    const candidates = [
      doc,
      document.documentElement,
      document.body,
      document.getElementById('main-content'),
      ...document.querySelectorAll('main, [class*="overflow-y"], [class*="overflow-auto"]'),
    ]

    let best = null
    let bestExtra = 0
    for (const el of candidates) {
      if (!el) continue
      const style = window.getComputedStyle(el)
      const oy = style.overflowY
      const canScroll = el === doc || oy === 'auto' || oy === 'scroll' || oy === 'overlay'
      const extra = el.scrollHeight - el.clientHeight
      if (canScroll && extra > bestExtra) {
        best = el
        bestExtra = extra
      }
    }

    if (docExtra >= bestExtra) {
      return { useWindow: true, scrollHeight: doc.scrollHeight, clientHeight: winH }
    }
    if (best && bestExtra > 40) {
      best.setAttribute('data-shot-scroll', '1')
      return { useWindow: false, scrollHeight: best.scrollHeight, clientHeight: best.clientHeight }
    }
    return { useWindow: true, scrollHeight: doc.scrollHeight, clientHeight: winH }
  })

  const isLong = metrics.scrollHeight > metrics.clientHeight + 48
  if (!isLong) {
    const dest = path.join(OUT_DIR, `${slug}.png`)
    await page.screenshot({ path: dest, fullPage: false })
    return [path.basename(dest)]
  }

  const overlap = 48
  const step = Math.max(80, metrics.clientHeight - overlap)
  const maxY = Math.max(0, metrics.scrollHeight - metrics.clientHeight)
  const files = []
  let y = 0
  let i = 1
  const maxSlices = 20

  while (i <= maxSlices) {
    if (metrics.useWindow) {
      await page.evaluate((yy) => window.scrollTo(0, yy), y)
    } else {
      await page.evaluate((yy) => {
        const el = document.querySelector('[data-shot-scroll="1"]')
        if (el) el.scrollTop = yy
      }, y)
    }
    await sleep(280)
    const dest = path.join(OUT_DIR, `${slug}_${i}.png`)
    await page.screenshot({ path: dest, fullPage: false })
    files.push(path.basename(dest))
    if (y >= maxY) break
    y = Math.min(maxY, y + step)
    i += 1
  }

  await page.evaluate((useWindow) => {
    if (useWindow) window.scrollTo(0, 0)
    else {
      const el = document.querySelector('[data-shot-scroll="1"]')
      if (el) {
        el.scrollTop = 0
        el.removeAttribute('data-shot-scroll')
      }
    }
  }, metrics.useWindow)

  return files
}

async function settle(page) {
  await page.waitForFunction(() => document.readyState === 'complete', { timeout: 10_000 }).catch(() => {})
  await sleep(600)
  await page
    .evaluate(async () => {
      if (document.fonts?.ready) await document.fonts.ready
    })
    .catch(() => {})
}

/**
 * Prove the injected session actually took before the shutter fires.
 *
 * A dashboard page rendered signed out is indistinguishable from a finished one:
 * the layout is complete, nothing errors, and the PNG publishes. The sidebar
 * footer prints the account's email once the user store is filled, so that string
 * is the evidence — and it identifies the account, not merely "somebody".
 *
 * Scoped to the sidebar, not the document: the admin users list and the
 * recent-signups list both print the screenshot account's own address in the page
 * body, so a whole-document match reports those two as signed in while the chrome
 * beside them is still the signed-out shell.
 *
 * textContent, not innerText: the sidebar is `hidden lg:flex`, so it contributes
 * no visible text at the mobile viewport while still sitting in the DOM.
 *
 * settle() is not enough on its own. It waits for readyState plus a fixed 600ms,
 * which lands before the client finishes /api/auth/refresh and /api/auth/me.
 */
async function waitForSignedIn(page, slug) {
  try {
    await page.waitForFunction(
      (email) => [...document.querySelectorAll('aside')].some((el) => el.textContent.includes(email)),
      { timeout: SESSION_TIMEOUT_MS },
      EMAIL,
    )
  } catch {
    sessionFailures.push(slug)
    const evidence = path.join(OUT_DIR, 'failures', `${slug}.png`)
    // Outside OUT_DIR's own glob so publish-to-site.sh can never pick it up.
    await mkdir(path.dirname(evidence), { recursive: true }).catch(() => {})
    await page.screenshot({ path: evidence, fullPage: false }).catch(() => {})
    throw new Error(`signed out after ${SESSION_TIMEOUT_MS}ms — ${EMAIL} never reached the page`)
  }
}

async function login(apiUrl, email, password) {
  const res = await fetch(`${apiUrl}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!res.ok) {
    const body = await res.text()
    console.warn(`login HTTP ${res.status}: ${body}`)
    return null
  }
  const data = await res.json()
  const accessToken =
    data.tokens?.accessToken ||
    data.access_token ||
    data.token?.access_token ||
    data.token?.accessToken
  const refreshToken =
    data.tokens?.refreshToken ||
    data.refresh_token ||
    data.token?.refresh_token ||
    data.token?.refreshToken
  if (!accessToken) {
    console.warn('login response missing access_token')
    return null
  }
  return { accessToken, refreshToken: refreshToken || '' }
}

async function enableExperimentalPrefs(apiUrl, accessToken) {
  const headers = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${accessToken}`,
  }
  let existing = {}
  try {
    const getRes = await fetch(`${apiUrl}/api/auth/preferences`, { headers })
    if (getRes.ok) {
      const data = await getRes.json()
      if (data.preferencesJson) existing = JSON.parse(data.preferencesJson)
    }
  } catch {
    // start from empty
  }
  const merged = {
    ...existing,
    experimental: {
      ...(existing.experimental ?? {}),
      whiteboardEnabled: true,
      youtubeEnabled: true,
      webxdcEnabled: true,
      whiteboardDisclaimerAcknowledged: true,
    },
    interface: { ...(existing.interface ?? {}), showWelcomeScreen: true },
  }
  const putRes = await fetch(`${apiUrl}/api/auth/preferences`, {
    method: 'PUT',
    headers,
    body: JSON.stringify({ preferencesJson: JSON.stringify(merged) }),
  })
  if (!putRes.ok) {
    console.warn(`could not save experimental prefs: HTTP ${putRes.status}`)
    return
  }
  console.log('enabled whiteboard / youtube / webxdc preferences')
}

async function createRoom(apiUrl, accessToken, name) {
  const res = await fetch(`${apiUrl}/api/room/create`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({ name, isPublic: true, maxParticipants: 20, mode: 'standard' }),
  })
  if (!res.ok) {
    console.warn(`create room HTTP ${res.status}: ${await res.text()}`)
    return false
  }
  return true
}

async function rasterizeKoboyoIcons(browser) {
  const slugs = [HOST_PERSON.icon, ...PEOPLE.map((p) => p.icon)]
  const out = {}
  const page = await browser.newPage()
  await page.setViewport({ width: 256, height: 256, deviceScaleFactor: 2 })
  try {
    for (const slug of slugs) {
      const svg = await loadIconSvg(slug)
      const html = `<!doctype html><html><body style="margin:0;background:#f4ece4">
        <div style="width:256px;height:256px;display:flex;align-items:center;justify-content:center;color:#1c1917">
          ${svg.replace('<svg ', '<svg width="200" height="200" ')}
        </div></body></html>`
      await page.setContent(html, { waitUntil: 'load' })
      const buf = await page.screenshot({ type: 'png', clip: { x: 0, y: 0, width: 256, height: 256 } })
      out[slug] = buf
      const pngPath = path.join(__dirname, 'avatars', `${slug}.png`)
      await writeFile(pngPath, buf).catch(() => {})
    }
  } finally {
    await page.close().catch(() => {})
  }
  return out
}

async function loadIconSvg(slug) {
  const local = path.join(__dirname, 'avatars', `${slug}.svg`)
  try {
    return await readFile(local, 'utf8')
  } catch {
    const res = await fetch(`https://koboyo.com/icons/svg/${slug}.svg`)
    if (!res.ok) throw new Error(`download ${slug}.svg: HTTP ${res.status}`)
    return res.text()
  }
}

async function ensurePersonSession(person, png) {
  await fetch(`${API_URL}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: person.email, password: PASSWORD || 'Screenshot1!', name: person.name }),
  }).catch(() => {})
  const session = await login(API_URL, person.email, PASSWORD || 'Screenshot1!')
  if (!session) {
    console.warn(`  could not log in ${person.name}`)
    return null
  }
  await setupPersonProfile(API_URL, session.accessToken, person.name, png)
  return session
}

async function setupPersonProfile(apiUrl, accessToken, name, png) {
  await fetch(`${apiUrl}/api/auth/me`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${accessToken}` },
    body: JSON.stringify({ name }),
  }).catch(() => {})
  if (!png) return
  const form = new FormData()
  form.append('avatar', new Blob([png], { type: 'image/png' }), 'avatar.png')
  const res = await fetch(`${apiUrl}/api/auth/me/avatar`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${accessToken}` },
    body: form,
  })
  if (!res.ok) console.warn(`  avatar upload for ${name}: HTTP ${res.status} ${await res.text()}`)
}

async function joinAsPerson(browser, roomName, person, png) {
  const session = await ensurePersonSession(person, png)
  if (!session) return null
  const context = await browser.createBrowserContext()
  await context.overridePermissions(BASE_URL, ['camera', 'microphone'])
  const page = await context.newPage()
  await page.setViewport({ width: 1280, height: 800, deviceScaleFactor: 1 })
  await page.evaluateOnNewDocument(
    ({ accessToken }) => {
      localStorage.setItem('theme', 'dark')
      localStorage.setItem(
        'interface-preferences',
        JSON.stringify({ state: { showWelcomeScreen: false }, version: 0 }),
      )
      localStorage.setItem('auth_remember', '1')
      localStorage.setItem('auth_at', accessToken)
    },
    { accessToken: session.accessToken },
  )
  const origin = new URL(BASE_URL)
  await page.setCookie(
    {
      name: 'access_token',
      value: session.accessToken,
      domain: origin.hostname,
      path: '/',
      httpOnly: true,
      sameSite: 'Lax',
    },
    {
      name: 'refresh_token',
      value: session.refreshToken,
      domain: origin.hostname,
      path: '/',
      httpOnly: true,
      sameSite: 'Lax',
    },
  )
  try {
    await page.goto(`${BASE_URL}/m/${roomName}`, { waitUntil: 'domcontentloaded', timeout: TIMEOUT_MS })
    await enterMeetingFromWelcome(page)
    await page.waitForSelector('.meet-tile', { timeout: TIMEOUT_MS })
    console.log(`  joined: ${person.name}`)
    return { context, page, guestName: person.name, line: person.line }
  } catch (err) {
    console.warn(`  ${person.name} failed: ${err.message}`)
    await page.close().catch(() => {})
    await context.close().catch(() => {})
    return null
  }
}

async function applyMeetingChrome(page, kind, extras = []) {
  if (!kind || kind === 'meeting') return

  if (kind === 'meeting-chat') {
    await fillMeetingChat(page, extras)
    return
  }
  if (kind === 'meeting-participants') {
    await clickLabeled(page, ['Show participants'])
    return
  }
  if (kind === 'meeting-info') {
    await clickLabeled(page, ['Show room info'])
    await sleep(400)
    return
  }
  if (kind === 'meeting-more') {
    await clickLabeled(page, ['More options'])
    await sleep(400)
    return
  }
  if (kind === 'meeting-settings') {
    await clickLabeled(page, ['More options'])
    await sleep(250)
    await clickButtonText(page, 'Settings')
    await sleep(400)
    await page.evaluate(() => {
      const tab = [...document.querySelectorAll('button, [role="tab"]')].find((el) => el.textContent.trim() === 'Experimental')
      tab?.click()
    })
    await sleep(400)
    return
  }
  if (kind === 'meeting-webxdc') {
    const opened = await clickLabeled(page, ['App gallery', 'App gallery (on stage)'])
    if (!opened) {
      await clickLabeled(page, ['More options'])
      await sleep(250)
      await clickButtonText(page, 'App gallery')
    }
    await sleep(700)
    return
  }
  if (kind === 'meeting-youtube') {
    await clickLabeled(page, ['More options'])
    await sleep(250)
    await clickButtonText(page, 'Share YouTube')
    await sleep(600)
    return
  }
  if (kind === 'meeting-whiteboard') {
    await clickLabeled(page, ['More options'])
    await sleep(250)
    await clickButtonText(page, 'Open whiteboard')
    await sleep(500)
    await clickButtonText(page, 'Continue')
    await sleep(1500)
    return
  }
  if (kind === 'meeting-screenshare') {
    await clickLabeled(page, ['Share screen'])
    await page
      .waitForFunction(
        () =>
          Boolean(
            document.querySelector('[aria-label="Stop screen share"], [aria-label="Stop sharing"]') ||
              document.querySelector('video'),
          ),
        { timeout: 15_000 },
      )
      .catch(() => {})
    await sleep(800)
  }
}

async function enterMeetingFromWelcome(page) {
  const cam = await page.$('button[aria-label="Turn camera on"]')
  if (cam) {
    await cam.click()
    await sleep(700)
  }
  const joined = await clickButtonText(page, 'Join meeting')
  if (!joined) {
    // Welcome already skipped — room should connect on its own.
    return
  }
  await sleep(400)
}

async function waitForPeople(page, minCount) {
  await page.waitForFunction(
    (n) => document.querySelectorAll('.meet-tile').length >= n,
    { timeout: TIMEOUT_MS },
    minCount,
  )
}

async function fillMeetingChat(hostPage, extras) {
  // Open chat on the host first so incoming LiveKit messages render here.
  await clickLabeled(hostPage, ['Open chat'])
  await hostPage
    .waitForSelector('textarea[placeholder="Type a message…"], textarea[aria-label="Chat message"]', {
      timeout: 8_000,
    })
    .catch(() => {})
  await sleep(300)

  const guests = extras.filter((e) => e?.page)
  for (const guest of guests) {
    await sendChatMessage(guest.page, guest.line || `Hi from ${guest.guestName}`)
  }

  await sendChatMessage(hostPage, 'Welcome in — starting in two minutes.')
  await sendChatMessage(hostPage, 'Drop questions here.')

  await hostPage
    .waitForFunction(
      () => {
        const text = document.body.innerText || ''
        return text.includes('hear me') || text.includes('Welcome in') || text.includes('loud and clear')
      },
      { timeout: 10_000 },
    )
    .catch(() => {})
  await sleep(500)
}

async function sendChatMessage(page, text) {
  if (!page) return
  const sel = 'textarea[placeholder="Type a message…"], textarea[aria-label="Chat message"]'
  try {
    let box = await page.$(sel)
    if (!box) {
      await clickLabeled(page, ['Open chat'])
      await page.waitForSelector(sel, { timeout: 8_000 })
      box = await page.$(sel)
    }
    if (!box) throw new Error('chat input not found')
    await box.click({ clickCount: 3 })
    await page.keyboard.press('Backspace')
    await page.keyboard.type(text, { delay: 12 })
    await page.keyboard.press('Enter')
    await sleep(400)
  } catch (err) {
    console.warn(`send chat skipped: ${err.message}`)
  }
}

async function clickButtonText(page, label) {
  return page.evaluate((wanted) => {
    const btn = [...document.querySelectorAll('button')].find((el) => el.textContent.trim() === wanted)
    if (!btn || btn.disabled) return false
    btn.click()
    return true
  }, label)
}

async function clickLabeled(page, prefixes) {
  return page.evaluate((starts) => {
    const btn = [...document.querySelectorAll('button')].find((el) => {
      const label = el.getAttribute('aria-label') || ''
      return starts.some((p) => label === p || label.startsWith(p))
    })
    if (!btn) return false
    btn.click()
    return true
  }, prefixes)
}

async function startStack() {
  const apiReady = await ping(API_URL).catch(() => false)
  if (!apiReady) {
    // Order matters: the Go binary embeds server/frontend at compile time, so the
    // bundle has to be written before the server is built.
    console.log('building the web bundle into server/frontend (bun run build:embed)…')
    await runToCompletion('bun', ['run', 'build:embed'], { cwd: path.join(REPO_ROOT, 'apps/web') })
    console.log('starting api (make dev-api)…')
    spawnManaged('make', ['dev-api'], { cwd: REPO_ROOT })
  } else {
    console.log(`api already listening on ${API_URL}`)
  }

  const lkReady = await ping(LIVEKIT_URL).catch(() => false)
  if (!lkReady) {
    console.log('starting livekit (make dev-livekit)…')
    spawnManaged('make', ['dev-livekit'], { cwd: REPO_ROOT })
  } else {
    console.log(`livekit already listening on ${LIVEKIT_URL}`)
  }
}

/** Run a build step to the end; a partial bundle would be embedded silently. */
function runToCompletion(cmd, cmdArgs, opts) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, cmdArgs, { ...opts, stdio: 'inherit' })
    child.on('error', reject)
    child.on('exit', (code) =>
      code === 0 ? resolve() : reject(new Error(`${cmd} ${cmdArgs.join(' ')} exited ${code}`)),
    )
  })
}

function spawnManaged(cmd, cmdArgs, opts) {
  const child = spawn(cmd, cmdArgs, {
    ...opts,
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: false,
  })
  children.push(child)
  child.stdout.on('data', (buf) => process.stdout.write(`[${cmd}] ${buf}`))
  child.stderr.on('data', (buf) => process.stderr.write(`[${cmd}] ${buf}`))
  child.on('exit', (code) => {
    const i = children.indexOf(child)
    if (i >= 0) children.splice(i, 1)
    if (code && code !== 0) {
      console.warn(`${cmd} exited ${code}`)
    }
  })
  return child
}

async function waitForHttp(url, timeoutMs) {
  const start = Date.now()
  let lastErr = ''
  while (Date.now() - start < timeoutMs) {
    try {
      const ok = await ping(url)
      if (ok) return
    } catch (err) {
      lastErr = err.message
    }
    await sleep(400)
  }
  throw new Error(`timed out waiting for ${url}${lastErr ? ` (${lastErr})` : ''}`)
}

async function ping(url) {
  const res = await fetch(url, { redirect: 'manual' })
  return res.status > 0
}

function shutdown(code) {
  for (const child of children) {
    try {
      child.kill('SIGTERM')
    } catch {
      // ignore
    }
  }
  process.exit(code)
}

function parseArgs(argv) {
  const out = {}
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    if (!a.startsWith('--')) continue
    const key = a.slice(2)
    const next = argv[i + 1]
    if (next == null || next.startsWith('--')) {
      out[key] = true
    } else {
      out[key] = next
      i++
    }
  }
  return out
}

function trimSlash(s) {
  return String(s).replace(/\/+$/, '')
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms))
}
