import { useEffect, useRef, useState } from 'react'
import { avatarInitials } from '#/components/meeting/chat/chatGrouping'
import { ParticipantAvatar } from '#/components/meeting/ParticipantAvatar'
import { API_URL } from '#/lib/api'
import { getPalette } from '#/lib/participant-palette'

export interface WelcomePresenceParticipant {
  identity: string
  name: string
  avatarUrl?: string
}

const PRESENCE_POLL_MS = 30_000
const AVATAR_RADIUS = 22
const MAX_PULL = 120
const LAUNCH_POWER = 0.14
const MAX_SPEED = 32
const WALL_RESTITUTION = 0.82
const BALL_RESTITUTION = 0.88
const AIR_DRAG = 0.9992
const MIN_DRIFT_SPEED = 1.15
const PHYSICS_SUBSTEPS = 5
const POSITION_SOLVER_ITERATIONS = 10
const TANGENTIAL_FRICTION = 0.12
const SLOP = 0.5

type PresenceBody = {
  identity: string
  x: number
  y: number
  vx: number
  vy: number
  radius: number
  active: boolean
}

type DragState = {
  identity: string
  pointerId: number
  originX: number
  originY: number
  currentX: number
  currentY: number
}

function presenceLayout(identity: string, index: number) {
  let hash = index * 17
  for (let i = 0; i < identity.length; i++) {
    hash = (hash * 31 + identity.charCodeAt(i)) >>> 0
  }
  const angle = ((hash % 360) * Math.PI) / 180
  const radiusX = 30 + (hash % 16)
  const radiusY = 26 + ((hash >> 4) % 14)
  const driftAngle = (((hash >> 3) % 360) * Math.PI) / 180
  const driftSpeed = 1.4 + ((hash >> 7) % 18) / 10
  return {
    xPct: 50 + Math.cos(angle) * radiusX,
    yPct: 46 + Math.sin(angle) * radiusY,
    vx: Math.cos(driftAngle) * driftSpeed,
    vy: Math.sin(driftAngle) * driftSpeed,
  }
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function constrainWallPosition(body: PresenceBody, width: number, height: number) {
  body.x = clamp(body.x, body.radius, width - body.radius)
  body.y = clamp(body.y, body.radius, height - body.radius)
}

function separateCirclePair(a: PresenceBody, b: PresenceBody) {
  let dx = b.x - a.x
  let dy = b.y - a.y
  let distSq = dx * dx + dy * dy
  const minDist = a.radius + b.radius - SLOP
  if (minDist <= 0) return

  if (distSq === 0) {
    dx = 1
    dy = 0
    distSq = 1
  }

  const dist = Math.sqrt(distSq)
  if (dist >= minDist) return

  const nx = dx / dist
  const ny = dy / dist
  const overlap = minDist - dist
  const half = overlap * 0.5

  a.x -= nx * half
  a.y -= ny * half
  b.x += nx * half
  b.y += ny * half
}

function applyWallImpulse(body: PresenceBody, width: number, height: number) {
  if (body.x <= body.radius + SLOP && body.vx < 0) {
    body.vx = -body.vx * WALL_RESTITUTION
  } else if (body.x >= width - body.radius - SLOP && body.vx > 0) {
    body.vx = -body.vx * WALL_RESTITUTION
  }

  if (body.y <= body.radius + SLOP && body.vy < 0) {
    body.vy = -body.vy * WALL_RESTITUTION
  } else if (body.y >= height - body.radius - SLOP && body.vy > 0) {
    body.vy = -body.vy * WALL_RESTITUTION
  }
}

function circlesTouching(a: PresenceBody, b: PresenceBody) {
  const dx = b.x - a.x
  const dy = b.y - a.y
  const minDist = a.radius + b.radius
  return dx * dx + dy * dy <= minDist * minDist
}

function applyCircleImpulse(a: PresenceBody, b: PresenceBody) {
  if (!circlesTouching(a, b)) return

  let dx = b.x - a.x
  let dy = b.y - a.y
  const distSq = dx * dx + dy * dy
  if (distSq === 0) {
    dx = 1
    dy = 0
  } else {
    const dist = Math.sqrt(distSq)
    dx /= dist
    dy /= dist
  }

  const nx = dx
  const ny = dy
  const tx = -ny
  const ty = nx

  const rvx = b.vx - a.vx
  const rvy = b.vy - a.vy
  const velNormal = rvx * nx + rvy * ny
  if (velNormal >= 0) return

  const normalImpulse = (-(1 + BALL_RESTITUTION) * velNormal) / 2
  a.vx -= normalImpulse * nx
  a.vy -= normalImpulse * ny
  b.vx += normalImpulse * nx
  b.vy += normalImpulse * ny

  const velTangent = rvx * tx + rvy * ty
  const tangentImpulse = (-velTangent * TANGENTIAL_FRICTION) / 2
  a.vx -= tangentImpulse * tx
  a.vy -= tangentImpulse * ty
  b.vx += tangentImpulse * tx
  b.vy += tangentImpulse * ty

  a.active = true
  b.active = true
}

function bodyIsContacting(body: PresenceBody, bodies: PresenceBody[], width: number, height: number) {
  if (
    body.x <= body.radius + 1 ||
    body.x >= width - body.radius - 1 ||
    body.y <= body.radius + 1 ||
    body.y >= height - body.radius - 1
  ) {
    return true
  }

  for (const other of bodies) {
    if (other.identity === body.identity) continue
    if (circlesTouching(body, other)) return true
  }

  return false
}

function applyMinDrift(body: PresenceBody, bodies: PresenceBody[], width: number, height: number) {
  if (bodyIsContacting(body, bodies, width, height)) return

  const speed = Math.hypot(body.vx, body.vy)
  if (speed >= MIN_DRIFT_SPEED) return

  const angle = speed > 0.05 ? Math.atan2(body.vy, body.vx) : (body.x + body.y) * 0.013
  body.vx = Math.cos(angle) * MIN_DRIFT_SPEED
  body.vy = Math.sin(angle) * MIN_DRIFT_SPEED
}

function simulatePhysics(bodies: PresenceBody[], width: number, height: number, drag: DragState | null) {
  const dt = 1 / PHYSICS_SUBSTEPS

  for (let step = 0; step < PHYSICS_SUBSTEPS; step++) {
    for (const body of bodies) {
      if (drag?.identity === body.identity) continue
      body.x += body.vx * dt
      body.y += body.vy * dt
    }

    for (let iter = 0; iter < POSITION_SOLVER_ITERATIONS; iter++) {
      for (const body of bodies) {
        if (drag?.identity === body.identity) continue
        constrainWallPosition(body, width, height)
      }

      for (let i = 0; i < bodies.length; i++) {
        for (let j = i + 1; j < bodies.length; j++) {
          if (drag?.identity === bodies[i].identity || drag?.identity === bodies[j].identity) continue
          separateCirclePair(bodies[i], bodies[j])
        }
      }
    }

    for (const body of bodies) {
      if (drag?.identity === body.identity) continue
      applyWallImpulse(body, width, height)
    }

    for (let i = 0; i < bodies.length; i++) {
      for (let j = i + 1; j < bodies.length; j++) {
        if (drag?.identity === bodies[i].identity || drag?.identity === bodies[j].identity) continue
        applyCircleImpulse(bodies[i], bodies[j])
      }
    }
  }

  for (const body of bodies) {
    if (drag?.identity === body.identity) continue
    body.vx *= AIR_DRAG
    body.vy *= AIR_DRAG
    applyMinDrift(body, bodies, width, height)
  }
}

function useWelcomeRoomPresence(roomId: string, enabled: boolean) {
  const [participants, setParticipants] = useState<WelcomePresenceParticipant[]>([])

  useEffect(() => {
    if (!enabled) {
      setParticipants([])
      return
    }

    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined

    async function refresh() {
      try {
        const res = await fetch(`${API_URL}/api/room/${encodeURIComponent(roomId)}/presence`, {
          credentials: 'include',
        })
        if (cancelled) return
        if (res.status === 429) {
          timer = setTimeout(refresh, 60_000)
          return
        }
        if (!res.ok) return
        const data = (await res.json()) as { participants?: WelcomePresenceParticipant[] }
        setParticipants(
          (data.participants ?? []).slice(0, 18).map((p) => ({
            identity: p.identity,
            name: p.name,
            avatarUrl: p.avatarUrl,
          })),
        )
        timer = setTimeout(refresh, PRESENCE_POLL_MS)
      } catch {
        if (!cancelled) timer = setTimeout(refresh, PRESENCE_POLL_MS)
      }
    }

    void refresh()
    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
    }
  }, [roomId, enabled])

  return participants
}

export function WelcomePresenceBackdrop({ roomId, enabled = true }: { roomId: string; enabled?: boolean }) {
  const participants = useWelcomeRoomPresence(roomId, enabled)
  const containerRef = useRef<HTMLDivElement>(null)
  const bodiesRef = useRef<Map<string, PresenceBody>>(new Map())
  const nodeRefs = useRef<Map<string, HTMLDivElement>>(new Map())
  const dragRef = useRef<DragState | null>(null)
  const aimLineRef = useRef<SVGLineElement>(null)
  const aimHeadRef = useRef<SVGPolygonElement>(null)
  const participantsRef = useRef(participants)
  participantsRef.current = participants

  useEffect(() => {
    if (!enabled) {
      bodiesRef.current.clear()
      return
    }

    const containerEl = containerRef.current
    if (!containerEl || participants.length === 0) return
    const el: HTMLDivElement = containerEl

    let raf = 0

    function syncParticipants() {
      const width = el.clientWidth
      const height = el.clientHeight
      if (width === 0 || height === 0) return

      const next = participantsRef.current
      const bodies = bodiesRef.current
      const ids = new Set(next.map((p) => p.identity))

      for (const id of [...bodies.keys()]) {
        if (!ids.has(id)) bodies.delete(id)
      }

      next.forEach((participant, index) => {
        if (bodies.has(participant.identity)) return
        const layout = presenceLayout(participant.identity, index)
        bodies.set(participant.identity, {
          identity: participant.identity,
          x: (layout.xPct / 100) * width,
          y: (layout.yPct / 100) * height,
          vx: layout.vx,
          vy: layout.vy,
          radius: AVATAR_RADIUS,
          active: true,
        })
      })
    }

    function updateAim() {
      const line = aimLineRef.current
      const head = aimHeadRef.current
      const drag = dragRef.current
      if (!line || !head) return

      if (!drag) {
        line.setAttribute('visibility', 'hidden')
        head.setAttribute('visibility', 'hidden')
        return
      }

      const body = bodiesRef.current.get(drag.identity)
      if (!body) return

      const pullX = drag.originX - drag.currentX
      const pullY = drag.originY - drag.currentY
      const pullLen = Math.hypot(pullX, pullY)
      const clampedLen = Math.min(pullLen, MAX_PULL)
      const scale = pullLen > 0 ? clampedLen / pullLen : 0
      const endX = body.x + pullX * scale
      const endY = body.y + pullY * scale

      line.setAttribute('visibility', 'visible')
      line.setAttribute('x1', String(body.x))
      line.setAttribute('y1', String(body.y))
      line.setAttribute('x2', String(endX))
      line.setAttribute('y2', String(endY))

      const launchX = body.x + pullX * scale * 1.35
      const launchY = body.y + pullY * scale * 1.35
      const angle = Math.atan2(body.y - endY, body.x - endX)
      const tipX = launchX
      const tipY = launchY
      const leftX = tipX - Math.cos(angle) * 10 + Math.sin(angle) * 5
      const leftY = tipY - Math.sin(angle) * 10 - Math.cos(angle) * 5
      const rightX = tipX - Math.cos(angle) * 10 - Math.sin(angle) * 5
      const rightY = tipY - Math.sin(angle) * 10 + Math.cos(angle) * 5

      head.setAttribute('visibility', 'visible')
      head.setAttribute('points', `${tipX},${tipY} ${leftX},${leftY} ${rightX},${rightY}`)
    }

    function tick() {
      const width = el.clientWidth
      const height = el.clientHeight
      if (width === 0 || height === 0) {
        raf = requestAnimationFrame(tick)
        return
      }

      syncParticipants()

      const bodies = [...bodiesRef.current.values()]
      const drag = dragRef.current

      simulatePhysics(bodies, width, height, drag)

      for (const body of bodies) {
        const node = nodeRefs.current.get(body.identity)
        if (!node) continue
        node.style.transform = `translate(${body.x}px, ${body.y}px) translate(-50%, -50%)`
        const isDragging = drag?.identity === body.identity
        node.style.opacity = body.active || isDragging ? '1' : '0.72'
        node.classList.toggle('meet-welcome-presence-avatar-active', body.active || isDragging)
        node.classList.toggle('meet-welcome-presence-avatar-dragging', isDragging)
      }

      updateAim()
      raf = requestAnimationFrame(tick)
    }

    function pointerPos(e: PointerEvent) {
      const rect = el.getBoundingClientRect()
      return {
        x: e.clientX - rect.left,
        y: e.clientY - rect.top,
      }
    }

    function onPointerDown(e: PointerEvent) {
      const target = e.target as HTMLElement | null
      const avatar = target?.closest('[data-presence-id]') as HTMLElement | null
      if (!avatar || !el.contains(avatar)) return

      const identity = avatar.dataset.presenceId
      if (!identity) return

      const body = bodiesRef.current.get(identity)
      if (!body) return

      e.preventDefault()
      avatar.setPointerCapture(e.pointerId)

      const pos = pointerPos(e)
      dragRef.current = {
        identity,
        pointerId: e.pointerId,
        originX: pos.x,
        originY: pos.y,
        currentX: pos.x,
        currentY: pos.y,
      }
      body.active = true
      body.vx = 0
      body.vy = 0
    }

    function onPointerMove(e: PointerEvent) {
      const drag = dragRef.current
      if (!drag || e.pointerId !== drag.pointerId) return
      e.preventDefault()
      const pos = pointerPos(e)
      drag.currentX = pos.x
      drag.currentY = pos.y
    }

    function onPointerUp(e: PointerEvent) {
      const drag = dragRef.current
      if (!drag || e.pointerId !== drag.pointerId) return

      const body = bodiesRef.current.get(drag.identity)
      if (body) {
        const pullX = drag.originX - drag.currentX
        const pullY = drag.originY - drag.currentY
        const pullLen = Math.min(Math.hypot(pullX, pullY), MAX_PULL)
        if (pullLen > 4) {
          const angle = Math.atan2(pullY, pullX)
          const speed = Math.min(pullLen * LAUNCH_POWER, MAX_SPEED)
          body.vx = Math.cos(angle) * speed
          body.vy = Math.sin(angle) * speed
          body.active = true
        }
      }

      dragRef.current = null
    }

    const resizeObserver = new ResizeObserver(() => {
      const width = el.clientWidth
      const height = el.clientHeight
      for (const body of bodiesRef.current.values()) {
        body.x = clamp(body.x, body.radius, Math.max(body.radius, width - body.radius))
        body.y = clamp(body.y, body.radius, Math.max(body.radius, height - body.radius))
      }
    })
    resizeObserver.observe(el)

    el.addEventListener('pointerdown', onPointerDown)
    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', onPointerUp)
    window.addEventListener('pointercancel', onPointerUp)

    raf = requestAnimationFrame(tick)

    return () => {
      cancelAnimationFrame(raf)
      resizeObserver.disconnect()
      el.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerup', onPointerUp)
      window.removeEventListener('pointercancel', onPointerUp)
      dragRef.current = null
    }
  }, [enabled, participants.length])

  if (!enabled) return null

  return (
    <div ref={containerRef} className="meet-welcome-presence absolute inset-0 z-[5] overflow-hidden" aria-hidden>
      <svg role="presentation" className="meet-welcome-presence-aim pointer-events-none absolute inset-0 h-full w-full">
        <line ref={aimLineRef} visibility="hidden" />
        <polygon ref={aimHeadRef} visibility="hidden" />
      </svg>
      {participants.map((participant) => {
        const palette = getPalette(participant.name || participant.identity)
        return (
          <div
            key={participant.identity}
            data-presence-id={participant.identity}
            ref={(el) => {
              if (el) nodeRefs.current.set(participant.identity, el)
              else nodeRefs.current.delete(participant.identity)
            }}
            className="meet-welcome-presence-avatar absolute left-0 top-0 touch-none select-none"
          >
            <ParticipantAvatar
              avatarUrl={participant.avatarUrl}
              initials={avatarInitials(participant.name || participant.identity)}
              paletteBackground={palette.avatar}
              className="h-11 w-11 text-[11px] shadow-[var(--meet-shadow)] ring-2 ring-[var(--meet-bg-panel)]"
            />
          </div>
        )
      })}
    </div>
  )
}
