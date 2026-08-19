#!/usr/bin/env node
/**
 * Live guest-join smoke: load SPA, fill name, join, assert API + CSS + UI.
 */
import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import puppeteer from 'puppeteer'

const BASE = process.env.BEDRUD_BASE_URL || 'http://127.0.0.1:18090'
const ROOM = process.env.BEDRUD_ROOM || 'sre-meetings'
const GUEST = process.env.BEDRUD_GUEST_NAME || 'Smoke Guest'
const OUT = process.env.BEDRUD_SCREENSHOT_OUT || '/tmp/bedrud-guest-smoke/output'

const failures = []
const notes = []

function fail(msg) {
  failures.push(msg)
  console.error('FAIL:', msg)
}

async function main() {
  await mkdir(OUT, { recursive: true })
  const browser = await puppeteer.launch({
    headless: true,
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || '/usr/bin/chromium',
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream'],
  })
  const page = await browser.newPage()
  await page.setViewport({ width: 1280, height: 800 })
  page.setDefaultTimeout(45_000)
  await page.browser().defaultBrowserContext().overridePermissions(BASE, ['camera', 'microphone'])

  const cssHits = []
  const apiHits = []
  page.on('response', (res) => {
    const url = res.url()
    const ct = res.headers()['content-type'] || ''
    if (url.includes('/assets/') && url.endsWith('.css')) {
      cssHits.push({ url, status: res.status(), ct })
    }
    if (url.includes('/api/room/guest-join') || url.includes('/api/auth/refresh') || url.includes('/api/room/create')) {
      apiHits.push({ url, status: res.status(), ct })
    }
  })
  page.on('console', (msg) => {
    if (msg.type() === 'error') notes.push(`console.error: ${msg.text()}`)
  })
  page.on('pageerror', (err) => notes.push(`pageerror: ${err.message}`))

  // 1) Landing / SPA shell
  const home = await page.goto(BASE + '/', { waitUntil: 'networkidle0' })
  if (!home || home.status() >= 400) fail(`GET / status ${home?.status()}`)
  await page.screenshot({ path: path.join(OUT, '01-home.png'), fullPage: true })

  const html = await page.content()
  if (!html.includes('root') && !html.includes('Bedrud') && html.length < 200) {
    fail('home HTML looks empty')
  }

  // CSS MIME — HTML for a stylesheet is the production failure mode.
  const cssOk = cssHits.filter((h) => h.status === 200 && h.ct.includes('text/css'))
  for (const hit of cssHits) {
    if (hit.ct.includes('text/html')) fail(`CSS ${hit.url} served as HTML (${hit.status})`)
    else if (hit.status === 200 && !hit.ct.includes('text/css')) fail(`CSS ${hit.url} content-type ${hit.ct}`)
    else notes.push(`CSS ${hit.status} ${hit.ct}`)
  }
  if (cssOk.length === 0) fail('no /assets/*.css loaded as text/css')

  // 2) Meeting guest dialog
  await page.goto(`${BASE}/m/${ROOM}`, { waitUntil: 'networkidle0' })
  await page.waitForFunction(() => document.body.innerText.includes('Join as guest') || document.body.innerText.includes('Joining'))
  await page.screenshot({ path: path.join(OUT, '02-guest-dialog.png') })

  const bodyText = await page.evaluate(() => document.body.innerText)
  if (!bodyText.includes('Join as guest')) {
    fail(`expected guest dialog, got: ${bodyText.slice(0, 400)}`)
  }

  const input = await page.waitForSelector('input')
  await input.click({ clickCount: 3 })
  await input.type(GUEST)
  await page.screenshot({ path: path.join(OUT, '03-guest-name-filled.png') })

  const joinBtn = await page.evaluateHandle(() => {
    const buttons = [...document.querySelectorAll('button')]
    return buttons.find((b) => (b.textContent || '').trim() === 'Join') || null
  })
  if (!joinBtn.asElement()) fail('Join button not found')
  else await joinBtn.asElement().click()

  // Wait for guest-join response or welcome / error
  await page.waitForFunction(
    () => {
      const t = document.body.innerText
      return (
        t.includes('Join meeting') ||
        t.includes('camera') ||
        t.includes('microphone') ||
        t.includes('Something went wrong') ||
        t.includes('Failed') ||
        t.includes('error') ||
        t.includes('Joining room')
      )
    },
    { timeout: 20_000 },
  ).catch(() => fail('timed out waiting after Join'))

  await new Promise((r) => setTimeout(r, 1500))
  await page.screenshot({ path: path.join(OUT, '04-after-join.png') })

  const guestJoin = apiHits.filter((h) => h.url.includes('/api/room/guest-join'))
  if (guestJoin.length === 0) fail('POST /api/room/guest-join never fired')
  for (const h of guestJoin) {
    notes.push(`guest-join ${h.status}`)
    if (h.status !== 200) fail(`guest-join HTTP ${h.status}`)
  }

  const after = await page.evaluate(() => document.body.innerText)
  if (after.includes('Failed to join') || after.includes('Failed to look up')) {
    fail(`error UI: ${after.slice(0, 500)}`)
  }
  if (after.includes('Join as guest') && guestJoin.some((h) => h.status === 200)) {
    fail('still on guest dialog after 200 guest-join')
  }

  // 3) Welcome prejoin → actually enter the LiveKit room
  if (!after.includes('Join meeting')) {
    fail(`expected welcome "Join meeting", got: ${after.slice(0, 400)}`)
  } else {
    const meetBtn = await page.evaluateHandle(() => {
      const buttons = [...document.querySelectorAll('button')]
      return buttons.find((b) => (b.textContent || '').trim() === 'Join meeting') || null
    })
    if (!meetBtn.asElement()) fail('Join meeting button not found')
    else await meetBtn.asElement().click()

    await page
      .waitForFunction(
        () => {
          const t = document.body.innerText
          const leave = document.querySelector('[aria-label="Leave meeting"]')
          return Boolean(
            leave ||
              t.includes('Leave') ||
              t.includes('Disconnected') ||
              t.includes('Failed') ||
              t.includes('could not connect') ||
              t.includes('Unable to connect'),
          )
        },
        { timeout: 40_000 },
      )
      .catch(() => fail('timed out waiting after Join meeting'))

    await new Promise((r) => setTimeout(r, 2500))
    await page.screenshot({ path: path.join(OUT, '05-in-meeting.png') })

    const inRoom = await page.evaluate(() => {
      const t = document.body.innerText
      return {
        t: t.slice(0, 1200),
        hasLeave: Boolean(document.querySelector('[aria-label="Leave meeting"]')),
        hasJoinMeeting: t.includes('Join meeting'),
        hasGuestDialog: t.includes('Join as guest'),
      }
    })
    notes.push(`in-meeting leave=${inRoom.hasLeave}`)
    if (inRoom.hasGuestDialog) fail('fell back to guest dialog after Join meeting')
    if (inRoom.hasJoinMeeting && !inRoom.hasLeave) fail('still on prejoin after Join meeting')
    if (!inRoom.hasLeave) fail(`did not enter meeting UI: ${inRoom.t.slice(0, 500)}`)
    if (/disconnected|unable to connect|could not connect|failed to connect/i.test(inRoom.t) && !inRoom.hasLeave) {
      fail(`connect error: ${inRoom.t.slice(0, 400)}`)
    }
  }

  const afterFinal = await page.evaluate(() => document.body.innerText)
  console.log(
    JSON.stringify(
      { ok: failures.length === 0, failures, notes, cssHits, apiHits, after: afterFinal.slice(0, 800) },
      null,
      2,
    ),
  )
  await browser.close()
  process.exit(failures.length ? 1 : 0)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
