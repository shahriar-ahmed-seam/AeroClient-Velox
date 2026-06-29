// TypeScript mirrors of the shared Go types in internal/model. Field names use
// camelCase to match the Go JSON tags so a value serializes identically across
// the Wails (desktop) and Capacitor (Android) bindings. These supersede the
// flat RequestState/ResponseState shapes in ./types.ts.

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS'
export type BodyType = 'none' | 'json' | 'text' | 'form-data' | 'urlencoded'
export type AuthType = 'none' | 'bearer' | 'basic' | 'apikey'
export type ApiKeyLocation = 'header' | 'query'
export type Theme = 'light' | 'dark' | 'system'

/** A generic enabled key/value pair (query params, headers, form fields). */
export interface KeyValue {
  key: string
  value: string
  enabled: boolean
}

/** Describes the request body and how it should be encoded. */
export interface BodySpec {
  type: BodyType
  raw: string // json/text content
  formFields: KeyValue[] // form-data / urlencoded
}

/** Describes the authorization configuration for a request. */
export interface AuthSpec {
  type: AuthType
  bearerToken: string
  basicUser: string
  basicPass: string
  apiKeyName: string
  apiKeyValue: string
  apiKeyLocation: ApiKeyLocation
}

/** A fully-configured but unsaved request as built in the editor. */
export interface RawRequest {
  method: Method
  url: string
  params: KeyValue[]
  headers: KeyValue[]
  body: BodySpec
  auth: AuthSpec
}

/** A RawRequest persisted within a Collection or Folder. Name is 1..255 chars. */
export interface SavedRequest extends RawRequest {
  id: string
  name: string
}

/** A nestable container for SavedRequests and other Folders (≤10 deep). */
export interface Folder {
  id: string
  name: string
  folders: Folder[]
  requests: SavedRequest[]
}

/** A named, ordered grouping of SavedRequests and Folders. */
export interface Collection {
  id: string
  name: string
  folders: Folder[]
  requests: SavedRequest[]
  order: number
}

/**
 * A named value referenced in requests via the {{name}} syntax. Name is 1..128
 * chars and unique within its Environment; value may be 0..4096 chars.
 */
export interface Variable {
  name: string
  value: string
}

/**
 * A named set of Variables. Name is 1..64 chars and unique among Environments.
 * At most one Environment is active at a time.
 */
export interface Environment {
  id: string
  name: string
  variables: Variable[]
  active: boolean
}

/** A persisted record of an executed request. error is "" on success. */
export interface HistoryEntry {
  id: string
  method: Method
  url: string
  status: number
  durationMs: number
  at: number
  error: string
  request: RawRequest // full config for restore
}

/**
 * User-configurable application preferences. Defaults are System theme, TLS
 * verification enabled, and a 30-second timeout.
 */
export interface Settings {
  theme: Theme // default "system"
  tlsVerify: boolean // default true
  timeoutSeconds: number // 1..600, default 30
  proxyUrl: string // stretch
}

/** Returned after executing a request. truncated is true when body > 5 MB. */
export interface HTTPResponse {
  status: number
  statusText: string
  headers: KeyValue[]
  body: string
  durationMs: number
  sizeBytes: number
  error: string
  truncated: boolean
}

/** Reports what an Import created so callers can surface or select the entry. */
export interface ImportResult {
  format: string // "collection" | "environment"
  collectionId?: string
  environmentId?: string
  name: string
}

// ---------------------------------------------------------------------------
// Factory helpers for empty/default values used by the editor and stores.
// ---------------------------------------------------------------------------

export function emptyKeyValue(): KeyValue {
  return { key: '', value: '', enabled: true }
}

export function emptyBodySpec(): BodySpec {
  return { type: 'none', raw: '', formFields: [] }
}

export function emptyAuthSpec(): AuthSpec {
  return {
    type: 'none',
    bearerToken: '',
    basicUser: '',
    basicPass: '',
    apiKeyName: '',
    apiKeyValue: '',
    apiKeyLocation: 'header',
  }
}

export function emptyRawRequest(): RawRequest {
  return {
    method: 'GET',
    url: '',
    params: [],
    headers: [],
    body: emptyBodySpec(),
    auth: emptyAuthSpec(),
  }
}

export function defaultSettings(): Settings {
  return { theme: 'system', tlsVerify: true, timeoutSeconds: 30, proxyUrl: '' }
}
