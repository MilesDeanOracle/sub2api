import { afterEach, describe, expect, it, vi } from 'vitest'
import { streamRun } from '../pelican'

vi.mock('../client', () => ({ apiClient: {} }))
vi.mock('@/i18n', () => ({ getLocale: () => 'zh' }))

function delta(value: Record<string, unknown>): string {
  return `data: ${JSON.stringify({ choices: [{ delta: value }] })}\n\n`
}

function mockStream(events: string[]): void {
  const encoder = new TextEncoder()
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const event of [...events, 'data: [DONE]\n\n']) {
        // Split UTF-8 characters and SSE blocks across network chunks.
        const bytes = encoder.encode(event)
        for (let i = 0; i < bytes.length; i += 7) controller.enqueue(bytes.slice(i, i + 7))
      }
      controller.close()
    }
  })
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, body }))
}

afterEach(() => vi.unstubAllGlobals())

describe('pelican stream reasoning and first token', () => {
  it('counts the first reasoning character once and keeps reasoning out of source', async () => {
    mockStream([
      ': connected\n\n',
      'event: pelican-meta\ndata: {"request_id":"request-1"}\n\n',
      delta({ role: 'assistant' }),
      delta({ reasoning_content: '', content: null }),
      delta({ reasoning_content: '先思考', reasoning: '不要重复' }),
      delta({ reasoning_content: '，再绘图。' }),
      delta({ content: '<html>动画</html>' }),
      'data: {"usage":{"completion_tokens":42}}\n\n'
    ])
    const events: string[] = []
    await streamRun({ model: 'test' }, {
      onFirstToken: () => events.push('first'),
      onReasoningDelta: (text) => events.push(`reasoning:${text}`),
      onDelta: (text) => events.push(`content:${text}`),
      onMeta: ({ requestId }) => events.push(`meta:${requestId}`),
      onUsage: (usage) => events.push(`usage:${usage.completion_tokens}`)
    })
    expect(events).toEqual([
      'meta:request-1', 'first', 'reasoning:先思考', 'reasoning:，再绘图。',
      'content:<html>动画</html>', 'usage:42'
    ])
  })

  it('supports the reasoning alias, including reasoning and content in the same event', async () => {
    mockStream([delta({ reasoning: '思考', content: '<html />' })])
    const onFirstToken = vi.fn()
    const onReasoningDelta = vi.fn()
    const onDelta = vi.fn()
    await streamRun({ model: 'test' }, { onFirstToken, onReasoningDelta, onDelta })
    expect(onFirstToken).toHaveBeenCalledTimes(1)
    expect(onReasoningDelta).toHaveBeenCalledWith('思考')
    expect(onDelta).toHaveBeenCalledWith('<html />')
  })

  it('falls back to content for models without reasoning', async () => {
    mockStream([delta({ content: '' }), delta({ content: '<html>' }), delta({ content: '</html>' })])
    const events: string[] = []
    await streamRun({ model: 'test' }, {
      onFirstToken: () => events.push('first'),
      onDelta: (text) => events.push(text)
    })
    expect(events).toEqual(['first', '<html>', '</html>'])
  })

  it('does not start the clock for empty deltas, metadata or usage', async () => {
    mockStream([
      'event: pelican-meta\ndata: {"request_id":"request-1"}\n\n',
      delta({ role: 'assistant', reasoning_content: '', reasoning: '', content: '' }),
      'data: {"usage":{"completion_tokens":0}}\n\n'
    ])
    const onFirstToken = vi.fn()
    await streamRun({ model: 'test' }, { onFirstToken, onDelta: vi.fn() })
    expect(onFirstToken).not.toHaveBeenCalled()
  })
})
