/**
 * Extract a single-file HTML animation from a model response and prepare it
 * for sandboxed rendering.
 *
 * The prompt asks the model to return raw HTML without a code fence, but this
 * module tolerates fenced output, leading explanations, SVG-only fragments and
 * truncated documents so a hiccup does not lose the whole result.
 */
export interface PelicanArtifact {
  /** Renderable HTML document, or null when the response is plain text. */
  html: string | null
  /** Text shown in the source tab (extracted HTML, or raw text as fallback). */
  source: string
  /** False when the document has no closing </html>. */
  complete: boolean
}

const FENCE_PATTERN = /```(?:html)?[ \t]*\r?\n([\s\S]*?)```/i
const CSP_META =
  `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; ` +
  `img-src data: blob:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; ` +
  `font-src data:; base-uri 'none'; form-action 'none'">`
const VIEWPORT_META = '<meta name="viewport" content="width=device-width, initial-scale=1">'

export function extractArtifact(raw: string): PelicanArtifact {
  const trimmed = raw.trim()
  if (!trimmed) {
    return { html: null, source: '', complete: false }
  }

  let candidate = trimmed
  const fenced = trimmed.match(FENCE_PATTERN)
  if (fenced && /<html|<svg|<!doctype/i.test(fenced[1])) {
    candidate = fenced[1].trim()
  }

  const lower = candidate.toLowerCase()
  const starts = [lower.indexOf('<!doctype'), lower.indexOf('<html')].filter((index) => index >= 0)
  if (starts.length === 0) {
    if (/<svg[\s>]/i.test(candidate)) {
      const html = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"></head><body>${candidate}</body></html>`
      return { html, source: html, complete: true }
    }
    return { html: null, source: candidate, complete: false }
  }

  const start = Math.min(...starts)
  const endIndex = lower.lastIndexOf('</html>')
  const complete = endIndex >= start
  const html = candidate.slice(start, complete ? endIndex + '</html>'.length : candidate.length).trim()

  return { html, source: html, complete }
}

/**
 * Prepare an extracted document for the preview iframe:
 * - inject a CSP that blocks network access but keeps inline CSS/SVG working
 * - ensure a viewport meta so responsive animations use the frame width
 * - remove navigation gadgets that could escape the sandbox
 * - propagate the host page's CSP nonce to inline scripts, because `srcdoc`
 *   inherits the host policy and the site's default `script-src` has no
 *   `'unsafe-inline'`. The nonce only lets the sandboxed artwork run its own
 *   animation code; it still cannot reach the parent or the network.
 */
export function prepareArtifactDocument(html: string, scriptNonce?: string, fitPreview = false): string {
  let doc = html
    .replace(/<base\b[^>]*>/gi, '')
    .replace(/<meta\b[^>]*http-equiv\s*=\s*(?:"refresh"|'refresh'|refresh)[^>]*>/gi, '')
    .replace(/<iframe\b[^>]*>[\s\S]*?<\/iframe>/gi, '')
    .replace(/<iframe\b[^>]*\/?>/gi, '')

  if (fitPreview) {
    // Fit inside a stable virtual viewport. Growing the iframe to scrollHeight
    // would also grow 100vh artwork and could keep clipping its bottom edge.
    const measurement = `<script>window.addEventListener('load',async()=>{
      await document.fonts?.ready;
      requestAnimationFrame(()=>requestAnimationFrame(()=>{
        const root=document.documentElement,body=document.body;
        let left=0,top=0,right=Math.max(root.scrollWidth,body?.scrollWidth||0,innerWidth),bottom=Math.max(root.scrollHeight,body?.scrollHeight||0,innerHeight);
        for(const element of body?.children||[]){
          const rect=element.getBoundingClientRect();
          left=Math.min(left,rect.left);top=Math.min(top,rect.top);right=Math.max(right,rect.right);bottom=Math.max(bottom,rect.bottom);
        }
        const width=right-left,height=bottom-top;
        const scale=Math.min(innerWidth/width,innerHeight/height);
        if(scale<1){
          root.style.setProperty('transform-origin','0 0','important');
          root.style.setProperty('transform','translate('+((innerWidth-width*scale)/2-left*scale)+'px,'+((innerHeight-height*scale)/2-top*scale)+'px) scale('+scale+')','important');
        }
      }));
    },{once:true});</script>`
    doc = /<\/body\s*>/i.test(doc) ? doc.replace(/<\/body\s*>/i, `${measurement}</body>`) : doc + measurement
  }

  const needsViewport = !/<meta\b[^>]*name\s*=\s*(?:"viewport"|'viewport'|viewport)/i.test(doc)
  const tags = needsViewport ? CSP_META + VIEWPORT_META : CSP_META

  const secured = applyScriptNonce(doc, scriptNonce)
  if (/<head\b[^>]*>/i.test(secured)) {
    return secured.replace(/<head\b([^>]*)>/i, `<head$1>${tags}`)
  }
  if (/<html\b[^>]*>/i.test(secured)) {
    return secured.replace(/<html\b([^>]*)>/i, `<html$1><head>${tags}</head>`)
  }
  return `<head>${tags}</head>${secured}`
}

function applyScriptNonce(html: string, nonce?: string): string {
  const value = nonce?.trim()
  if (!value) {
    return html
  }
  const escaped = value.replace(/"/g, '&quot;')
  return html.replace(/<script\b(?![^>]*\bnonce=)/gi, `<script nonce="${escaped}"`)
}
