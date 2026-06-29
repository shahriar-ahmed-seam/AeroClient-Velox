// Small, pure presentation helpers shared across components: method/status
// color mapping, byte formatting, and best-effort JSON pretty-printing.
//
// The application data model lives in ./models.ts (mirroring internal/model);
// this module holds only stateless view helpers, so it is safe to import from
// any component without pulling in stores or the backend.

/** Color token for an HTTP method label. */
export function methodColor(method: string): string {
  switch (method) {
    case 'GET': return 'var(--green)'
    case 'POST': return 'var(--accent)'
    case 'PUT': return 'var(--blue)'
    case 'PATCH': return 'var(--purple)'
    case 'DELETE': return 'var(--red)'
    default: return 'var(--text-dim)'
  }
}

/** Color token for an HTTP status code, by class: 2xx/3xx/4xx/5xx, else dim. */
export function statusColor(status: number): string {
  if (status >= 200 && status < 300) return 'var(--green)'
  if (status >= 300 && status < 400) return 'var(--blue)'
  if (status >= 400 && status < 500) return 'var(--accent)'
  if (status >= 500 && status < 600) return 'var(--red)'
  return 'var(--text-dim)'
}

/** Human-readable byte size (B / KB / MB). */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
}

/** Pretty-print a string as 2-space JSON, returning it unchanged if invalid. */
export function prettyMaybeJSON(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}
