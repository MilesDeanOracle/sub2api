import { describe, expect, it } from 'vitest'
import { extractArtifact, prepareArtifactDocument } from '../artifact'

describe('extractArtifact', () => {
  it('extracts raw HTML and drops surrounding explanations', () => {
    const result = extractArtifact('下面是代码：\n<!DOCTYPE html><html><body><svg></svg></body></html>\n希望你喜欢')
    expect(result.complete).toBe(true)
    expect(result.html).toContain('<!DOCTYPE html>')
    expect(result.html).toContain('</html>')
    expect(result.html).not.toContain('希望你喜欢')
  })

  it('unwraps a fenced html block even though the prompt forbids it', () => {
    const result = extractArtifact('```html\n<!doctype html><html><body>ok</body></html>\n```')
    expect(result.complete).toBe(true)
    expect(result.html).toBe('<!doctype html><html><body>ok</body></html>')
  })

  it('wraps an SVG-only fragment in a minimal document', () => {
    const result = extractArtifact('<svg viewBox="0 0 10 10"><circle cx="5" cy="5" r="4" /></svg>')
    expect(result.complete).toBe(true)
    expect(result.html).toContain('<!doctype html>')
    expect(result.html).toContain('<svg')
  })

  it('marks a truncated document as incomplete', () => {
    const result = extractArtifact('<html><body><div>cut off')
    expect(result.complete).toBe(false)
    expect(result.html).toContain('<div>cut off')
  })

  it('returns plain text without pretending it is HTML', () => {
    const result = extractArtifact('抱歉，我无法生成这个动画。')
    expect(result.html).toBeNull()
    expect(result.source).toContain('抱歉')
  })
})

describe('prepareArtifactDocument', () => {
  it('injects CSP and viewport into head', () => {
    const doc = prepareArtifactDocument('<!doctype html><html><head><title>x</title></head><body></body></html>')
    expect(doc).toContain("default-src 'none'")
    expect(doc).toContain('name="viewport"')
    expect(doc).toContain('<title>x</title>')
  })

  it('does not duplicate an existing viewport meta', () => {
    const doc = prepareArtifactDocument(
      '<!doctype html><html><head><meta name="viewport" content="width=device-width"></head><body></body></html>'
    )
    expect(doc.match(/name="viewport"/g)).toHaveLength(1)
  })

  it('removes base, refresh and iframe gadgets but keeps scripts and styles', () => {
    const doc = prepareArtifactDocument(
      '<!doctype html><html><head><base href="https://evil.test"><meta http-equiv="refresh" content="0;url=https://evil.test"></head>' +
        '<body><style>.a{animation:x 1s infinite}</style><script>requestAnimationFrame(()=>{})</script>' +
        '<iframe src="https://evil.test"></iframe><svg></svg></body></html>'
    )
    expect(doc).not.toContain('<base')
    expect(doc).not.toContain('http-equiv="refresh"')
    expect(doc).not.toContain('<iframe')
    expect(doc).toContain('animation:x 1s infinite')
    expect(doc).toContain('requestAnimationFrame')
  })

  it('adds a head when the document has only an html tag', () => {
    const doc = prepareArtifactDocument('<html><body><svg></svg></body></html>')
    expect(doc).toContain('<head>')
    expect(doc).toContain("default-src 'none'")
  })

  it('forwards the host CSP nonce to inline scripts without duplicating it', () => {
    const doc = prepareArtifactDocument(
      '<html><body><script>requestAnimationFrame(()=>{})</script><script nonce="keep">x()</script></body></html>',
      'host-nonce'
    )
    expect(doc).toContain('<script nonce="host-nonce">requestAnimationFrame')
    expect(doc).toContain('<script nonce="keep">x()')
    expect(doc.match(/nonce="host-nonce"/g)).toHaveLength(1)
  })
})
