<template>
  <canvas
    ref="canvasRef"
    class="agent-avatar"
    :class="{ rounded }"
    :style="hostStyle"
    :width="canvasPixelSize"
    :height="canvasPixelSize"
    role="img"
    :aria-label="ariaLabel"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { AVATAR_SCREEN_RECTS } from '../../constants/avatarScreens'
import { ensureAvatarSprite, getAvatarSprite } from '../../utils/avatarSprite'
import type { SessionStatusTone } from '../../utils/sessionStatus'

// public/avatar.png is a 250×200 sheet laid out 5 cols × 4 rows = 20
// cells, native 50×50. Indices go left-to-right, top-to-bottom; keep in
// sync with avatarSpriteCount in core/internal/session/session.go.
const SPRITE_COLS = 5
const SPRITE_ROWS = 4
const SPRITE_CELL_PX = 50
const SPRITE_TOTAL = SPRITE_COLS * SPRITE_ROWS

// Corner radius approximating each bot's screen silhouette. Used to
// clip every state overlay so the rounded corners are preserved.
const SCREEN_CLIP_RADIUS_SPRITE_PX = 6

// Monospace stack used by the state overlay text. Canvas can't read CSS
// variables, so we hardcode something close to var(--mono).
const MONO_STACK = 'ui-monospace, "SF Mono", Menlo, monospace'

// — Flicker (running state) config —
const FLICKER_ALPHABET = '01<>{}()/[]:;=+*&^%$#@!?aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789'
const FLICKER_LEN = 96
const FLICKER_SWAP_INTERVAL_MS = 120
const FLICKER_SWAPS_MIN = 3
const FLICKER_SWAPS_MAX = 7

// — Blink config —
const BLINK_PERIOD_MS = 4400
// Phases inside one period (0..1) — eyelids retracted everywhere except
// in the closing → closed → opening band right at the end.
const BLINK_CLOSE_START = 0.92
const BLINK_FULLY_CLOSED_START = 0.95
const BLINK_FULLY_CLOSED_END = 0.97

// — Approval (pulsing !) config —
const APPROVAL_PULSE_PERIOD_MS = 1300
const APPROVAL_PULSE_MIN_SCALE = 0.85
const APPROVAL_PULSE_MAX_SCALE = 1.10

// — Skill install: rainbow wave drag-over —
// HSL palette mirroring the card's drop-target gradient
// (#d7b7b2 → #c7a7c4 → #ada4d3 → #72c0d0 → #8bc59a → #d7b37d → #d7b7b2).
// We use it as a horizontal stroke gradient on a single sine wave, with
// a time-shifted hue offset reproducing the `hue-rotate` effect.
const RAINBOW_PALETTE_HSL: ReadonlyArray<{ h: number; s: number; l: number }> = [
  { h: 11, s: 32, l: 77 },   // muted pink
  { h: 311, s: 24, l: 72 },  // muted lavender
  { h: 247, s: 33, l: 73 },  // soft purple
  { h: 190, s: 47, l: 63 },  // soft cyan
  { h: 135, s: 33, l: 66 },  // soft green
  { h: 34, s: 49, l: 67 },   // soft amber
  { h: 11, s: 32, l: 77 },   // back to start
]
// Hue rotation period — matches the card's drop-target shimmer.
const RAINBOW_HUE_PERIOD_MS = 1000
// Wave travel period — how long it takes one full wavelength to slide
// across the screen.
const RAINBOW_WAVE_PHASE_PERIOD_MS = 1400
// Wave geometry, expressed as fractions of the screen rect.
const RAINBOW_WAVE_AMP_RATIO = 0.28      // peak-to-axis as a fraction of screen height
const RAINBOW_WAVE_STROKE_RATIO = 0.22   // stroke thickness as a fraction of screen height
const RAINBOW_WAVE_CYCLES = 1.5          // visible sine cycles across the screen width
// Path subdivision count — enough that the curve stays smooth at any
// avatar size we render (largest screen ~50 px even at avatar=72).
const RAINBOW_WAVE_SEGMENTS = 32

// — Skill install: EXP+ flash —
const FLASH_TEXT = 'EXP+'
const FLASH_PULSE_PERIOD_MS = 600
const FLASH_PULSE_MIN_SCALE = 0.95
const FLASH_PULSE_MAX_SCALE = 1.05

// — Manual eyelid close (for "eyes shut" states like installing) —
// How long the bot takes to close / open its eyes when entering or
// leaving a manual-closed state. 240 ms reads as a deliberate gesture
// without holding up the rest of the animation.
const MANUAL_EYELID_TRANSITION_MS = 240

const props = withDefaults(defineProps<{
  /** Sprite index (string or number). Falsy values fall back to 0. */
  index?: string | number
  /** Rendered side length in CSS pixels. */
  size?: number
  /** Round the avatar into a circle. Defaults to a soft square. */
  rounded?: boolean
  ariaLabel?: string
  /** Disable the idle blink animation. Useful in dense pickers where
   *  20 simultaneous blinks would be visual noise. */
  disableBlink?: boolean
  /** Lifecycle state driving the screen overlay:
   *    'idle'     — eyes visible, idle blink only.
   *    'running'  — black screen + scrolling code chars + in-place swaps.
   *    'approval' — black screen + pulsing exclamation mark.
   *    'error'    — currently same as idle.
   *  Map directly from `getSessionStatusTone(session)` at the call site. */
  state?: SessionStatusTone
  /** True while a skill is being dragged over the card hosting this
   *  avatar. The screen rect lights up with a hue-cycling pastel
   *  rainbow that matches the card's drop-target shimmer. Takes
   *  precedence over `state` so the user gets unambiguous feedback. */
  installing?: boolean
  /** True briefly (typically ~1.5s, parent-controlled) right after a
   *  skill install succeeds. Renders an "EXP+" flash on the screen.
   *  Higher priority than `installing` so a quick second drag can't
   *  step on the celebration frame. */
  installFlash?: boolean
}>(), {
  index: 0,
  size: 32,
  rounded: false,
  ariaLabel: 'Agent avatar',
  disableBlink: false,
  state: 'idle',
  installing: false,
  installFlash: false,
})

const canvasRef = ref<HTMLCanvasElement | null>(null)

// Snapshot DPR once at mount; re-snapshotting on display changes is a
// nice-to-have we can add later if it actually trips someone up.
const dpr = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1
const canvasPixelSize = computed(() => Math.round(props.size * dpr))
const hostStyle = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
}))

const resolvedIndex = computed(() => {
  const raw = typeof props.index === 'string' ? parseInt(props.index, 10) : props.index
  if (!Number.isFinite(raw) || raw < 0) return 0
  return Math.floor(raw) % SPRITE_TOTAL
})

const isRunning = computed(() => props.state === 'running')
const isApproval = computed(() => props.state === 'approval')
const isInstalling = computed(() => props.installing)
const isInstallFlash = computed(() => props.installFlash)

const reducedMotion = typeof window !== 'undefined'
  && window.matchMedia('(prefers-reduced-motion: reduce)').matches

// Per-instance random offsets so a grid of avatars desyncs on every
// animation. Computed once at setup.
const blinkDelayMs = Math.random() * 3000
const flickerDurationMs = 5000 + Math.random() * 2000
const flickerStartOffsetMs = Math.random() * flickerDurationMs

// Flicker text: mutable buffer that mutates in place every
// FLICKER_SWAP_INTERVAL_MS while running. Read directly inside the rAF
// render — no Vue reactivity needed since the canvas redraws every
// frame anyway.
const flickerChars: string[] = Array.from({ length: FLICKER_LEN }, () => randomFlickerChar())

function randomFlickerChar(): string {
  return FLICKER_ALPHABET[Math.floor(Math.random() * FLICKER_ALPHABET.length)]
}

let flickerSwapTimer: ReturnType<typeof setInterval> | null = null

function tickFlickerSwap() {
  const span = FLICKER_SWAPS_MAX - FLICKER_SWAPS_MIN + 1
  const swaps = FLICKER_SWAPS_MIN + Math.floor(Math.random() * span)
  for (let i = 0; i < swaps; i += 1) {
    flickerChars[Math.floor(Math.random() * flickerChars.length)] = randomFlickerChar()
  }
}

function startFlickerSwap() {
  if (flickerSwapTimer != null) return
  flickerSwapTimer = setInterval(tickFlickerSwap, FLICKER_SWAP_INTERVAL_MS)
}

function stopFlickerSwap() {
  if (flickerSwapTimer != null) {
    clearInterval(flickerSwapTimer)
    flickerSwapTimer = null
  }
}

watch(
  isRunning,
  (running) => {
    if (running) startFlickerSwap()
    else stopFlickerSwap()
  },
  { immediate: true },
)

// — Manual eyelid closure (drives the "eyes shut during installing"
// gesture). Idle blink continues independently and the renderer takes
// the max of the two so a blink can't visually pry the eyes open while
// they're "manually" held shut.
let manualEyelidClosure = 0
let manualEyelidTransition: {
  from: number
  to: number
  startT: number
  durationMs: number
} | null = null

watch(isInstalling, (installing) => {
  const target = installing ? 1 : 0
  if (target === manualEyelidClosure && !manualEyelidTransition) return
  if (reducedMotion) {
    manualEyelidClosure = target
    manualEyelidTransition = null
    return
  }
  const startedAt = startTime === 0 ? 0 : performance.now() - startTime
  manualEyelidTransition = {
    from: manualEyelidClosure,
    to: target,
    startT: startedAt,
    durationMs: MANUAL_EYELID_TRANSITION_MS,
  }
})

function tickManualEyelid(t: number) {
  const trans = manualEyelidTransition
  if (!trans) return
  const elapsed = t - trans.startT
  if (elapsed >= trans.durationMs) {
    manualEyelidClosure = trans.to
    manualEyelidTransition = null
    return
  }
  const phase = elapsed / trans.durationMs
  manualEyelidClosure = trans.from + (trans.to - trans.from) * easeInOut(phase)
}

// — rAF render loop —

let rafId = 0
let startTime = 0
// Cached glyph advance for the current font size. measureText is cheap
// but doing it once per resize beats 60× per second.
let cachedFontSize = -1
let cachedCharWidth = 0

onMounted(async () => {
  startTime = performance.now()
  try {
    await ensureAvatarSprite()
  } catch (err) {
    console.warn('[AgentAvatar] sprite load failed', err)
  }
  startRender()
})

onBeforeUnmount(() => {
  if (rafId !== 0) cancelAnimationFrame(rafId)
  stopFlickerSwap()
})

function startRender() {
  function frame() {
    render()
    rafId = requestAnimationFrame(frame)
  }
  rafId = requestAnimationFrame(frame)
}

function render() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const t = performance.now() - startTime
  const size = props.size

  // Reset transform → clear in raw pixel space → re-apply DPR scale and
  // disable smoothing (the canvas resets its state when width/height
  // attributes change, so we re-apply every frame defensively).
  ctx.setTransform(1, 0, 0, 1, 0, 0)
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  ctx.scale(dpr, dpr)
  ctx.imageSmoothingEnabled = false

  // 1. Bot sprite (whole 50×50 cell scaled to `size`).
  const sprite = getAvatarSprite()
  if (sprite) {
    const idx = resolvedIndex.value
    const col = idx % SPRITE_COLS
    const row = Math.floor(idx / SPRITE_COLS)
    ctx.drawImage(
      sprite,
      col * SPRITE_CELL_PX, row * SPRITE_CELL_PX, SPRITE_CELL_PX, SPRITE_CELL_PX,
      0, 0, size, size,
    )
  }

  // 2. State overlays clipped to the rounded screen rect.
  const rect = AVATAR_SCREEN_RECTS[resolvedIndex.value]
  if (!rect) return

  const s = size / SPRITE_CELL_PX
  const screen = {
    x: rect.x * s,
    y: rect.y * s,
    w: rect.w * s,
    h: rect.h * s,
    r: SCREEN_CLIP_RADIUS_SPRITE_PX * s,
  }

  tickManualEyelid(t)

  // Priority order (highest first): the success flash takes precedence
  // so a rapid second drag can't smother the celebration; install
  // drag-over takes precedence over agent lifecycle state so the user
  // gets unambiguous feedback during the gesture.
  if (isInstallFlash.value) {
    drawInstallFlash(ctx, t, screen)
  } else if (manualEyelidClosure > 0) {
    // Installing — or animating in/out of installing. Eyes close (or
    // are closed), then the rainbow wave is drawn on top with its
    // alpha tied to the closure so it fades in/out alongside the eyes.
    const idle = computeIdleBlinkClosure(t)
    drawEyelids(ctx, screen, Math.max(manualEyelidClosure, idle))
    ctx.save()
    ctx.globalAlpha = manualEyelidClosure
    drawRainbow(ctx, t, screen)
    ctx.restore()
  } else if (isRunning.value) {
    drawFlicker(ctx, t, screen)
  } else if (isApproval.value) {
    drawApproval(ctx, t, screen)
  } else {
    // Plain idle: just the time-driven blink.
    drawEyelids(ctx, screen, computeIdleBlinkClosure(t))
  }
}

// — Draw helpers —

interface ScreenRect {
  x: number
  y: number
  w: number
  h: number
  r: number
}

function clipScreen(ctx: CanvasRenderingContext2D, s: ScreenRect) {
  ctx.beginPath()
  ctx.roundRect(s.x, s.y, s.w, s.h, s.r)
  ctx.clip()
}

function drawFlicker(ctx: CanvasRenderingContext2D, t: number, s: ScreenRect) {
  ctx.save()
  clipScreen(ctx, s)

  // Solid black backdrop covering the eyes.
  ctx.fillStyle = '#000'
  ctx.fillRect(s.x, s.y, s.w, s.h)

  const fontSize = Math.max(6, s.h * 0.85)
  ctx.font = `600 ${fontSize}px ${MONO_STACK}`
  if (cachedFontSize !== fontSize) {
    cachedCharWidth = ctx.measureText('M').width
    cachedFontSize = fontSize
  }
  ctx.fillStyle = '#34d36b'
  ctx.shadowColor = 'rgba(52, 211, 107, 0.55)'
  ctx.shadowBlur = 2
  ctx.textBaseline = 'middle'
  ctx.textAlign = 'left'

  const text = flickerChars.join('')
  const textW = text.length * cachedCharWidth

  // Continuous right→left scroll. reducedMotion freezes the offset at 0
  // so the screen still reads as "running" without the moving chars.
  const phase = reducedMotion
    ? 0
    : ((t + flickerStartOffsetMs) % flickerDurationMs) / flickerDurationMs
  const offset = -phase * textW

  // Draw twice for seamless wrap — the right edge of the first copy
  // hands off cleanly to the left edge of the second.
  ctx.fillText(text, s.x + offset, s.y + s.h / 2)
  ctx.fillText(text, s.x + offset + textW, s.y + s.h / 2)

  ctx.restore()
}

function drawApproval(ctx: CanvasRenderingContext2D, t: number, s: ScreenRect) {
  ctx.save()
  clipScreen(ctx, s)

  ctx.fillStyle = '#000'
  ctx.fillRect(s.x, s.y, s.w, s.h)

  const fontSize = Math.max(7, s.h * 1.05)
  ctx.font = `700 ${fontSize}px ${MONO_STACK}`
  ctx.fillStyle = '#fbbf24'
  ctx.shadowColor = 'rgba(251, 191, 36, 0.6)'
  ctx.shadowBlur = 3
  ctx.textBaseline = 'middle'
  ctx.textAlign = 'center'

  // Sine-driven scale between APPROVAL_PULSE_MIN_SCALE and MAX_SCALE.
  // Frozen at the midpoint when reduced-motion is on.
  let scale = (APPROVAL_PULSE_MIN_SCALE + APPROVAL_PULSE_MAX_SCALE) / 2
  if (!reducedMotion) {
    const phase = (t % APPROVAL_PULSE_PERIOD_MS) / APPROVAL_PULSE_PERIOD_MS
    const sineUnit = Math.sin(phase * Math.PI * 2) * 0.5 + 0.5 // 0..1
    scale = APPROVAL_PULSE_MIN_SCALE
      + (APPROVAL_PULSE_MAX_SCALE - APPROVAL_PULSE_MIN_SCALE) * sineUnit
  }

  ctx.translate(s.x + s.w / 2, s.y + s.h / 2)
  ctx.scale(scale, scale)
  ctx.fillText('!', 0, 0)

  ctx.restore()
}

// Time-driven idle blink closure in [0, 1]. Returns 0 when blinking is
// disabled / reduced motion is on / a state overlay covers the screen,
// so callers can blindly Math.max it with a manual closure.
function computeIdleBlinkClosure(t: number): number {
  if (props.disableBlink || reducedMotion) return 0
  if (isInstallFlash.value || isRunning.value || isApproval.value) return 0
  const phase = ((t + blinkDelayMs) % BLINK_PERIOD_MS) / BLINK_PERIOD_MS
  if (phase < BLINK_CLOSE_START) return 0
  if (phase < BLINK_FULLY_CLOSED_START) {
    return easeInOut((phase - BLINK_CLOSE_START)
      / (BLINK_FULLY_CLOSED_START - BLINK_CLOSE_START))
  }
  if (phase < BLINK_FULLY_CLOSED_END) return 1
  return easeInOut(1 - (phase - BLINK_FULLY_CLOSED_END)
    / (1 - BLINK_FULLY_CLOSED_END))
}

// Draws two black eyelids growing from the top and bottom of the screen
// rect to a combined closure in [0, 1]. closure=0 leaves the eyes
// fully visible; closure=1 closes them with a 0.5 px overlap so no
// stray bright pixel bleeds between the lids.
function drawEyelids(
  ctx: CanvasRenderingContext2D,
  s: ScreenRect,
  closure: number,
) {
  if (closure <= 0) return
  const halfH = s.h / 2 + 0.5
  const eyelidH = halfH * closure
  ctx.save()
  clipScreen(ctx, s)
  ctx.fillStyle = '#000'
  ctx.fillRect(s.x, s.y, s.w, eyelidH)
  ctx.fillRect(s.x, s.y + s.h - eyelidH, s.w, eyelidH)
  ctx.restore()
}

function drawRainbow(ctx: CanvasRenderingContext2D, t: number, s: ScreenRect) {
  ctx.save()
  clipScreen(ctx, s)

  // Two independent time loops:
  //   • huePhase rotates the gradient palette so the wave's colours
  //     cycle through the full hue circle every RAINBOW_HUE_PERIOD_MS.
  //   • wavePhase slides the sine peaks rightward across the screen
  //     every RAINBOW_WAVE_PHASE_PERIOD_MS.
  // reducedMotion freezes both — the static rainbow wave still reads
  // as "drop target", just without the motion.
  const huePhase = reducedMotion ? 0 : (t % RAINBOW_HUE_PERIOD_MS) / RAINBOW_HUE_PERIOD_MS
  const hueShift = huePhase * 360
  const wavePhase = reducedMotion
    ? 0
    : ((t % RAINBOW_WAVE_PHASE_PERIOD_MS) / RAINBOW_WAVE_PHASE_PERIOD_MS) * Math.PI * 2

  // Gradient stroke: horizontal across the screen rect so each stop
  // paints a fixed band of the wave regardless of where its peaks sit.
  const grad = ctx.createLinearGradient(s.x, 0, s.x + s.w, 0)
  for (let i = 0; i < RAINBOW_PALETTE_HSL.length; i += 1) {
    const c = RAINBOW_PALETTE_HSL[i]
    const h = (c.h + hueShift) % 360
    grad.addColorStop(i / (RAINBOW_PALETTE_HSL.length - 1), `hsl(${h}, ${c.s}%, ${c.l}%)`)
  }

  ctx.strokeStyle = grad
  ctx.lineWidth = Math.max(1.5, s.h * RAINBOW_WAVE_STROKE_RATIO)
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'

  // Sine wave traced as a polyline across the screen rect.
  const cy = s.y + s.h / 2
  const amp = s.h * RAINBOW_WAVE_AMP_RATIO
  const k = (RAINBOW_WAVE_CYCLES * Math.PI * 2) / s.w // angular freq per displayed px
  ctx.beginPath()
  for (let i = 0; i <= RAINBOW_WAVE_SEGMENTS; i += 1) {
    const u = (i / RAINBOW_WAVE_SEGMENTS) * s.w
    const x = s.x + u
    const y = cy + Math.sin(u * k - wavePhase) * amp
    if (i === 0) ctx.moveTo(x, y)
    else ctx.lineTo(x, y)
  }
  ctx.stroke()

  ctx.restore()
}

function drawInstallFlash(ctx: CanvasRenderingContext2D, t: number, s: ScreenRect) {
  ctx.save()
  clipScreen(ctx, s)

  ctx.fillStyle = '#000'
  ctx.fillRect(s.x, s.y, s.w, s.h)

  // Shrink the glyph until "EXP+" actually fits. measureText runs every
  // frame but the layout is stable for any given size, so we'd cache
  // it if this ever showed up in profiles — for now it's negligible.
  const baseFontSize = Math.max(7, s.h * 0.85)
  ctx.font = `700 ${baseFontSize}px ${MONO_STACK}`
  const measured = ctx.measureText(FLASH_TEXT).width
  const maxWidth = s.w * 0.9
  const fontSize = measured > maxWidth ? baseFontSize * (maxWidth / measured) : baseFontSize

  ctx.font = `700 ${fontSize}px ${MONO_STACK}`
  ctx.fillStyle = '#fbbf24'
  ctx.shadowColor = 'rgba(251, 191, 36, 0.7)'
  ctx.shadowBlur = 3
  ctx.textBaseline = 'middle'
  ctx.textAlign = 'center'

  // Subtle pulse so the flash feels alive without competing with the
  // bigger approval pulse animation.
  let scale = (FLASH_PULSE_MIN_SCALE + FLASH_PULSE_MAX_SCALE) / 2
  if (!reducedMotion) {
    const phase = (t % FLASH_PULSE_PERIOD_MS) / FLASH_PULSE_PERIOD_MS
    const sineUnit = Math.sin(phase * Math.PI * 2) * 0.5 + 0.5
    scale = FLASH_PULSE_MIN_SCALE
      + (FLASH_PULSE_MAX_SCALE - FLASH_PULSE_MIN_SCALE) * sineUnit
  }
  ctx.translate(s.x + s.w / 2, s.y + s.h / 2)
  ctx.scale(scale, scale)
  ctx.fillText(FLASH_TEXT, 0, 0)

  ctx.restore()
}

function easeInOut(x: number): number {
  return x < 0.5 ? 2 * x * x : 1 - Math.pow(-2 * x + 2, 2) / 2
}
</script>

<style scoped>
.agent-avatar {
  display: inline-block;
  flex-shrink: 0;
  border-radius: 6px;
  /* Pixel-art rendering on the canvas surface itself. The drawImage
   * call additionally sets imageSmoothingEnabled=false on the 2d
   * context so the sprite stays crisp under non-integer scale factors. */
  image-rendering: pixelated;
  image-rendering: crisp-edges;
}

.agent-avatar.rounded {
  border-radius: 50%;
}
</style>
