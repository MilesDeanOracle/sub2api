const CONTAINER_ID = 'GTM-PQ6C6RBB'
const SCRIPT_ID = 'google-tag-manager'

declare global {
  interface Window {
    dataLayer?: unknown[]
  }
}

export function initGoogleTagManager(): void {
  if (document.getElementById(SCRIPT_ID)) return

  window.dataLayer = window.dataLayer || []
  window.dataLayer.push({ 'gtm.start': Date.now(), event: 'gtm.js' })

  const script = document.createElement('script')
  script.id = SCRIPT_ID
  script.async = true
  script.src = `https://www.googletagmanager.com/gtm.js?id=${CONTAINER_ID}`

  // GTM propagates this nonce to tags it loads, including the Google tag.
  const nonce = document.querySelector<HTMLScriptElement>('script[nonce]')?.nonce
  if (nonce) script.nonce = nonce

  document.head.appendChild(script)
}
