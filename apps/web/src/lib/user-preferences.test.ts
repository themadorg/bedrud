import { describe, expect, it } from 'vitest'
import { parseUserPreferences } from './user-preferences'

describe('user-preferences', () => {
  it('parses preference blobs safely', () => {
    expect(parseUserPreferences('{"audio":{"pushToTalkEnabled":true}}')).toEqual({
      audio: { pushToTalkEnabled: true },
    })
    expect(parseUserPreferences('')).toEqual({})
    expect(parseUserPreferences('[]')).toEqual({})
  })
})
