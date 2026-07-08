/** Strong LTR letters (Latin, etc.) — mirrors Excalidraw's isRTL helper. */
const LTR_CHARS =
  'A-Za-z\u00C0-\u00D6\u00D8-\u00F6\u00F8-\u02B8\u0300-\u0590\u0800-\u1FFF' +
  '\u2C00-\uFB1C\uFDFE-\uFE6F\uFEFD-\uFFFF'
const RTL_CHARS = '\u0591-\u07FF\uFB1D-\uFDFD\uFE70-\uFEFC'
const STARTS_RTL = new RegExp(`^[^${LTR_CHARS}]*[${RTL_CHARS}]`)

/** True when the first strong directional character is RTL (Persian, Arabic, Hebrew, …). */
export function textStartsRtl(text: string): boolean {
  return STARTS_RTL.test(text)
}

export function textDirectionFor(text: string): 'rtl' | 'ltr' {
  return textStartsRtl(text) ? 'rtl' : 'ltr'
}