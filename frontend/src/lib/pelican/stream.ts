/**
 * Minimal Server-Sent Events reader for the pelican run endpoint.
 *
 * The backend passes the upstream OpenAI SSE stream through unchanged, so the
 * parser only needs to split events and join `data:` lines. It deliberately
 * does not depend on EventSource (which cannot send Authorization headers).
 */
export interface SSEMessage {
  event: string
  data: string
}

export async function readSSE(
  body: ReadableStream<Uint8Array>,
  onMessage: (message: SSEMessage) => void
): Promise<void> {
  const reader = body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) {
        break
      }
      buffer += decoder.decode(value, { stream: true })
      buffer = buffer.replace(/\r\n/g, '\n')
      let boundary = buffer.indexOf('\n\n')
      while (boundary >= 0) {
        const block = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)
        const message = parseBlock(block)
        if (message) {
          onMessage(message)
        }
        boundary = buffer.indexOf('\n\n')
      }
    }

    buffer += decoder.decode()
    if (buffer.trim()) {
      const message = parseBlock(buffer)
      if (message) {
        onMessage(message)
      }
    }
  } finally {
    reader.releaseLock()
  }
}

function parseBlock(block: string): SSEMessage | null {
  let event = 'message'
  const data: string[] = []

  for (const line of block.split('\n')) {
    if (!line || line.startsWith(':')) {
      continue
    }
    const colon = line.indexOf(':')
    const field = colon >= 0 ? line.slice(0, colon) : line
    let value = colon >= 0 ? line.slice(colon + 1) : ''
    if (value.startsWith(' ')) {
      value = value.slice(1)
    }
    if (field === 'event') {
      event = value
    } else if (field === 'data') {
      data.push(value)
    }
  }

  if (data.length === 0) {
    return null
  }
  return { event, data: data.join('\n') }
}
