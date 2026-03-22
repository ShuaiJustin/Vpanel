import { describe, expect, it } from 'vitest'

import { sanitizeHtml } from './htmlSanitizer'

describe('sanitizeHtml', () => {
  it('removes blocked tags and inline event handlers', () => {
    const input = '<div onclick="alert(1)">safe<script>alert(1)</script><img src="/ok.png" onerror="hack()"></div>'
    const output = sanitizeHtml(input)

    expect(output).toContain('<div>safe<img src="/ok.png"></div>')
    expect(output).not.toContain('<script')
    expect(output).not.toContain('onclick')
    expect(output).not.toContain('onerror')
  })

  it('removes dangerous URLs and hardens target blank links', () => {
    const input = '<a href="javascript:alert(1)" target="_blank">bad</a><a href="https://example.com" target="_blank">good</a>'
    const output = sanitizeHtml(input)

    expect(output).toContain('<a target="_blank" rel="noopener noreferrer">bad</a>')
    expect(output).toContain('href="https://example.com"')
    expect(output).toContain('rel="noopener noreferrer"')
  })
})
