/**
 * Contract tests read stylesheets, the web manifest and icon bytes straight from disk. The
 * app's tsconfig leaves Node's type package out on purpose, because its globals (for
 * example `setTimeout`) collide with the browser's, so the one Node module those tests use
 * is declared here with only the shapes they call.
 */
declare module 'node:fs' {
  /** The part of Node's Buffer the tests need to read a PNG's dimensions. */
  export interface FileBytes {
    readUInt32BE(offset: number): number
  }

  export function readFileSync(path: string | URL, encoding: 'utf8'): string
  export function readFileSync(path: string | URL): FileBytes
}
