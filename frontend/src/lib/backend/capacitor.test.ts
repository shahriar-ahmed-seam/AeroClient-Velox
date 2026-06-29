// Unit tests for capacitorBackend's JSON-envelope handling. The native plugin
// (VoltBridge) is mocked, so these run in the web/jsdom test environment without
// the AAR. They verify the contract capacitorBackend relies on:
//   - execute returns the HTTPResponse verbatim, preserving its `error` field
//     (Req 15.4) rather than throwing on it;
//   - CRUD/list methods return parsed typed data on success;
//   - the {"error":...} envelope is thrown as an Error;
//   - action-only methods accept {"ok":true} as void;
//   - migrateLegacyHistory unwraps {"migrated":bool};
//   - args are marshalled to the JSON string shape the plugin expects.

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { emptyRawRequest } from '../models'

// Mock the native plugin module. Each method is a vi.fn resolving { result }.
const bridge = {
  execute: vi.fn(),
  saveCollection: vi.fn(),
  renameCollection: vi.fn(),
  deleteCollection: vi.fn(),
  saveFolder: vi.fn(),
  deleteFolder: vi.fn(),
  saveRequest: vi.fn(),
  deleteRequest: vi.fn(),
  moveRequest: vi.fn(),
  listTree: vi.fn(),
  saveEnvironment: vi.fn(),
  deleteEnvironment: vi.fn(),
  setActiveEnvironment: vi.fn(),
  listEnvironments: vi.fn(),
  addHistory: vi.fn(),
  listHistory: vi.fn(),
  clearHistory: vi.fn(),
  migrateLegacyHistory: vi.fn(),
  getSettings: vi.fn(),
  saveSettings: vi.fn(),
  exportCollection: vi.fn(),
  exportEnvironment: vi.fn(),
  importData: vi.fn(),
  appVersion: vi.fn(),
}

vi.mock('../native/voltBridgePlugin', () => ({ VoltBridge: bridge }))

// Import after the mock is registered.
const { capacitorBackend } = await import('./capacitor')

beforeEach(() => {
  vi.clearAllMocks()
})

describe('capacitorBackend.executeRequest', () => {
  it('returns the HTTPResponse verbatim and preserves its error field (Req 15.4)', async () => {
    const resp = {
      status: 0,
      statusText: '',
      headers: [],
      body: '',
      durationMs: 0,
      sizeBytes: 0,
      error: 'dial tcp: connection refused',
      truncated: false,
    }
    bridge.execute.mockResolvedValue({ result: JSON.stringify(resp) })

    const req = emptyRawRequest()
    const out = await capacitorBackend.executeRequest(req)

    // The request is marshalled to JSON for the plugin.
    expect(bridge.execute).toHaveBeenCalledWith({ reqJSON: JSON.stringify(req) })
    // The error in the HTTPResponse is preserved, not thrown.
    expect(out.error).toBe('dial tcp: connection refused')
    expect(out.status).toBe(0)
  })

  it('returns a successful response unchanged', async () => {
    const resp = {
      status: 200,
      statusText: 'OK',
      headers: [{ key: 'X', value: 'y', enabled: true }],
      body: '{}',
      durationMs: 12,
      sizeBytes: 2,
      error: '',
      truncated: false,
    }
    bridge.execute.mockResolvedValue({ result: JSON.stringify(resp) })

    const out = await capacitorBackend.executeRequest(emptyRawRequest())
    expect(out.status).toBe(200)
    expect(out.headers).toEqual([{ key: 'X', value: 'y', enabled: true }])
  })
})

describe('capacitorBackend CRUD success', () => {
  it('listTree parses the JSON array result', async () => {
    const tree = [{ id: 'c1', name: 'Coll', folders: [], requests: [], order: 0 }]
    bridge.listTree.mockResolvedValue({ result: JSON.stringify(tree) })

    const out = await capacitorBackend.listTree()
    expect(out).toEqual(tree)
  })

  it('saveCollection marshals input and parses the saved collection', async () => {
    const input = { id: '', name: 'New', folders: [], requests: [], order: 0 }
    const saved = { ...input, id: 'c9' }
    bridge.saveCollection.mockResolvedValue({ result: JSON.stringify(saved) })

    const out = await capacitorBackend.saveCollection(input)
    expect(bridge.saveCollection).toHaveBeenCalledWith({ collectionJSON: JSON.stringify(input) })
    expect(out.id).toBe('c9')
  })

  it('migrateLegacyHistory unwraps the {migrated} envelope', async () => {
    bridge.migrateLegacyHistory.mockResolvedValue({ result: JSON.stringify({ migrated: true }) })
    const out = await capacitorBackend.migrateLegacyHistory([])
    expect(out).toBe(true)
  })

  it('appVersion returns the plain result string', async () => {
    bridge.appVersion.mockResolvedValue({ result: '1.4.2' })
    expect(await capacitorBackend.appVersion()).toBe('1.4.2')
  })

  it('exportCollection returns the export JSON string on success', async () => {
    const envelope = '{"voltFormat":"collection","version":1,"collection":{}}'
    bridge.exportCollection.mockResolvedValue({ result: envelope })
    expect(await capacitorBackend.exportCollection('c1')).toBe(envelope)
  })
})

describe('capacitorBackend error-envelope handling', () => {
  it('throws the Bridge error for a data method', async () => {
    bridge.saveCollection.mockResolvedValue({ result: '{"error":"name too long"}' })
    await expect(
      capacitorBackend.saveCollection({ id: '', name: '', folders: [], requests: [], order: 0 }),
    ).rejects.toThrow('name too long')
  })

  it('throws the Bridge error for a list method', async () => {
    bridge.listEnvironments.mockResolvedValue({ result: '{"error":"db locked"}' })
    await expect(capacitorBackend.listEnvironments()).rejects.toThrow('db locked')
  })

  it('throws the Bridge error for an export method', async () => {
    bridge.exportCollection.mockResolvedValue({ result: '{"error":"not found"}' })
    await expect(capacitorBackend.exportCollection('missing')).rejects.toThrow('not found')
  })

  it('throws on malformed JSON from the bridge', async () => {
    bridge.listTree.mockResolvedValue({ result: 'not json' })
    await expect(capacitorBackend.listTree()).rejects.toThrow(/malformed JSON/)
  })
})

describe('capacitorBackend action-only methods', () => {
  it('resolves void on {"ok":true}', async () => {
    bridge.renameCollection.mockResolvedValue({ result: '{"ok":true}' })
    await expect(capacitorBackend.renameCollection('c1', 'New')).resolves.toBeUndefined()
    expect(bridge.renameCollection).toHaveBeenCalledWith({ id: 'c1', name: 'New' })
  })

  it('throws on the error envelope for an action-only method', async () => {
    bridge.deleteCollection.mockResolvedValue({ result: '{"error":"no such collection"}' })
    await expect(capacitorBackend.deleteCollection('x')).rejects.toThrow('no such collection')
  })

  it('marshals moveRequest args to the plugin field names', async () => {
    bridge.moveRequest.mockResolvedValue({ result: '{"ok":true}' })
    await capacitorBackend.moveRequest('r1', 'p2')
    expect(bridge.moveRequest).toHaveBeenCalledWith({ requestID: 'r1', targetParentID: 'p2' })
  })
})
