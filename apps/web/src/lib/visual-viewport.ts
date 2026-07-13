/**
 * iOS Safari layout viewport can be larger than the *visible* viewport when the
 * URL / toolbar chrome is shown. `100vh`/`100vw`, `fixed; top/left: 50%`, and
 * `inset-0` use the layout viewport — so modals clip off-screen.
 *
 * Fix: publish the Visual Viewport API as CSS vars and size/position fixed UI
 * against those instead of raw vh/vw units.
 *
 *   --app-height       visible height in px (fallback: 100svh)
 *   --app-width        visible width in px  (fallback: 100svw)
 *   --app-offset-top   visualViewport.offsetTop
 *   --app-offset-left  visualViewport.offsetLeft
 *
 * @see https://developer.mozilla.org/en-US/docs/Web/API/Visual_Viewport_API
 * @see https://bugs.webkit.org/show_bug.cgi?id=141832
 */

const HEIGHT_VAR = '--app-height'
const WIDTH_VAR = '--app-width'
const OFFSET_TOP_VAR = '--app-offset-top'
const OFFSET_LEFT_VAR = '--app-offset-left'

function readVisualViewport(): {
  height: number
  width: number
  offsetTop: number
  offsetLeft: number
} {
  const vv = window.visualViewport
  if (vv) {
    return {
      height: vv.height,
      width: vv.width,
      offsetTop: vv.offsetTop,
      offsetLeft: vv.offsetLeft,
    }
  }
  return {
    height: window.innerHeight,
    width: window.innerWidth,
    offsetTop: 0,
    offsetLeft: 0,
  }
}

function px(n: number): string {
  return `${Math.round(n * 100) / 100}px`
}

export function updateAppViewportCssVars(): void {
  if (typeof document === 'undefined') return
  const { height, width, offsetTop, offsetLeft } = readVisualViewport()
  const root = document.documentElement
  root.style.setProperty(HEIGHT_VAR, px(height))
  root.style.setProperty(WIDTH_VAR, px(width))
  root.style.setProperty(OFFSET_TOP_VAR, px(offsetTop))
  root.style.setProperty(OFFSET_LEFT_VAR, px(offsetLeft))
}

/** Subscribe to visualViewport + window resize. Returns cleanup. */
export function installVisualViewportCssVars(): () => void {
  updateAppViewportCssVars()

  const onChange = () => updateAppViewportCssVars()
  const vv = window.visualViewport
  vv?.addEventListener('resize', onChange)
  vv?.addEventListener('scroll', onChange)
  window.addEventListener('resize', onChange)
  window.addEventListener('orientationchange', onChange)

  return () => {
    vv?.removeEventListener('resize', onChange)
    vv?.removeEventListener('scroll', onChange)
    window.removeEventListener('resize', onChange)
    window.removeEventListener('orientationchange', onChange)
  }
}
