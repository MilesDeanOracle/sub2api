import { describe, expect, it } from 'vitest'
import { readSSE, type SSEMessage } from '../stream'

function streamFrom(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder()
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk))
      }
      controller.close()
    }
  })
}

async function collect(chunks: string[]): Promise<SSEMessage[]> {
  const messages: SSEMessage[] = []
  await readSSE(streamFrom(chunks), (message) => messages.push(message))
  return messages
}

describe('readSSE', () => {
  it('parses data-only events split across chunks', async () => {
    const messages = await collect(['data: {"a"', ':1}\n\n', 'data: [DONE]\n\n'])
    expect(messages).toEqual([
      { event: 'message', data: '{"a":1}' },
      { event: 'message', data: '[DONE]' }
    ])
  })

  it('supports CRLF, multi-line data, comments and named events', async () => {
    const messages = await collect([
      ': connected\r\n\r\n',
      'event: pelican-error\r\n',
      'data: {"code":"X",\r\n',
      'data: "message":"y"}\r\n\r\n'
    ])
    expect(messages).toEqual([
      { event: 'pelican-error', data: '{"code":"X",\n"message":"y"}' }
    ])
  })

  it('flushes a final event without a trailing blank line', async () => {
    const messages = await collect(['data: tail'])
    expect(messages).toEqual([{ event: 'message', data: 'tail' }])
  })

  it('ignores blocks without data', async () => {
    const messages = await collect(['event: ping\n\n', 'data: value\n\n'])
    expect(messages).toEqual([{ event: 'message', data: 'value' }])
  })
})
