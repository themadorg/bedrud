/** Detect WebXDC package files for upload / drag-drop (require .xdc name). */
export function isWebxdcFile(file: File): boolean {
  return file.name.toLowerCase().endsWith('.xdc')
}

export function pickWebxdcFileFromDataTransfer(dt: DataTransfer | null): File | null {
  if (!dt?.files?.length) return null
  for (const file of Array.from(dt.files)) {
    if (isWebxdcFile(file)) return file
  }
  return null
}
