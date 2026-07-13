import { describe, expect, it } from 'vitest'
import {
  isTrustedWebxdcMessageEvent,
  parseWebxdcIframeMessage,
} from './webxdcHostMessage'
import { WEBXDC_POSTMESSAGE_CHANNEL } from './webxdcConstants'

describe('parseWebxdcIframeMessage', () => {
  const appId = 'instance-1'

  it('accepts sendUpdate for bound appId', () => {
    const msg = parseWebxdcIframeMessage(
      {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'sendUpdate',
        appId,
        update: { payload: { ok: true } },
      },
      appId,
    )
    expect(msg?.type).toBe('sendUpdate')
  })

  it('rejects wrong channel', () => {
    expect(
      parseWebxdcIframeMessage(
        { channel: 'other', type: 'ready', appId },
        appId,
      ),
    ).toBeNull()
  })

  it('rejects cross-appId publish', () => {
    expect(
      parseWebxdcIframeMessage(
        {
          channel: WEBXDC_POSTMESSAGE_CHANNEL,
          type: 'sendUpdate',
          appId: 'other-app',
          update: { payload: 1 },
        },
        appId,
      ),
    ).toBeNull()
  })

  it('rejects invalid update payload', () => {
    expect(
      parseWebxdcIframeMessage(
        {
          channel: WEBXDC_POSTMESSAGE_CHANNEL,
          type: 'sendUpdate',
          appId,
          update: { href: 'https://x' },
        },
        appId,
      ),
    ).toBeNull()
  })

  it('parses setUpdateListener with serial', () => {
    const msg = parseWebxdcIframeMessage(
      {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'setUpdateListener',
        appId,
        serial: 7,
      },
      appId,
    )
    expect(msg).toEqual({
      channel: WEBXDC_POSTMESSAGE_CHANNEL,
      type: 'setUpdateListener',
      appId,
      serial: 7,
    })
  })

  it('defaults setUpdateListener serial to 0', () => {
    const msg = parseWebxdcIframeMessage(
      {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'setUpdateListener',
        appId,
      },
      appId,
    )
    expect(msg?.type).toBe('setUpdateListener')
    if (msg?.type === 'setUpdateListener') expect(msg.serial).toBe(0)
  })

  it('parses getUpdates pull requests', () => {
    const msg = parseWebxdcIframeMessage(
      {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'getUpdates',
        appId,
        requestId: 'u-1',
        after: 3,
      },
      appId,
    )
    expect(msg).toEqual({
      channel: WEBXDC_POSTMESSAGE_CHANNEL,
      type: 'getUpdates',
      appId,
      requestId: 'u-1',
      after: 3,
    })
  })

  it('accepts ready without appId (early ping)', () => {
    const msg = parseWebxdcIframeMessage(
      { channel: WEBXDC_POSTMESSAGE_CHANNEL, type: 'ready' },
      appId,
    )
    expect(msg?.type).toBe('ready')
    expect(msg && 'appId' in msg ? msg.appId : null).toBe(appId)
  })

  it('parses sendToChat and openExternal', () => {
    const chat = parseWebxdcIframeMessage(
      {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'sendToChat',
        appId,
        requestId: 'r1',
        text: 'hi',
        file: null,
      },
      appId,
    )
    expect(chat?.type).toBe('sendToChat')
    const ext = parseWebxdcIframeMessage(
      {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'openExternal',
        appId,
        url: 'https://example.com/x',
      },
      appId,
    )
    expect(ext?.type).toBe('openExternal')
  })

  it('parses rtJoin and rtSend', () => {
    expect(
      parseWebxdcIframeMessage(
        { channel: WEBXDC_POSTMESSAGE_CHANNEL, type: 'rtJoin', appId },
        appId,
      )?.type,
    ).toBe('rtJoin')
    const send = parseWebxdcIframeMessage(
      {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'rtSend',
        appId,
        data: [1, 2, 255],
      },
      appId,
    )
    expect(send?.type).toBe('rtSend')
    if (send?.type === 'rtSend') expect(send.data).toEqual([1, 2, 255])
  })

  it('rejects rtSend with invalid bytes', () => {
    expect(
      parseWebxdcIframeMessage(
        {
          channel: WEBXDC_POSTMESSAGE_CHANNEL,
          type: 'rtSend',
          appId,
          data: [1, 300],
        },
        appId,
      ),
    ).toBeNull()
  })
})

describe('isTrustedWebxdcMessageEvent', () => {
  const source = {} as MessageEventSource

  it('requires matching origin and source', () => {
    expect(
      isTrustedWebxdcMessageEvent(
        { origin: 'https://webxdc.example', source },
        'https://webxdc.example',
        source,
      ),
    ).toBe(true)
    expect(
      isTrustedWebxdcMessageEvent(
        { origin: 'https://evil.example', source },
        'https://webxdc.example',
        source,
      ),
    ).toBe(false)
    expect(
      isTrustedWebxdcMessageEvent(
        { origin: 'https://webxdc.example', source: null },
        'https://webxdc.example',
        source,
      ),
    ).toBe(false)
  })
})
