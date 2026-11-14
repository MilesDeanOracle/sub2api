import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PelicanArtifactPreview from '@/components/pelican/PelicanArtifactPreview.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
afterEach(() => vi.restoreAllMocks())

describe('Operations artifact isolation', () => {
  it('keeps generated scripts sandboxed when fullscreen is unavailable', async () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mount(PelicanArtifactPreview, { props: { html: '<html><body><script>window.parent.location="/"</script></body></html>', allowNewTab: false, embedded: true } })
    const frame = wrapper.get('iframe')
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
    expect(frame.attributes('srcdoc')).toContain("default-src 'none'")
    expect(wrapper.text()).not.toContain('pelican.openNewTab')
    await wrapper.findAll('button').find(b => b.text() === 'pelican.fullscreen')!.trigger('click')
    await flushPromises()
    expect(open).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('scales the virtual viewport on stage resize including fullscreen', async () => {
    let resize!: () => void
    const disconnect = vi.fn()
    vi.stubGlobal('ResizeObserver', class {
      constructor(callback: () => void) { resize = callback }
      observe() {}
      disconnect = disconnect
    })
    const wrapper = mount(PelicanArtifactPreview, { props: { html: '<html><body>large scene</body></html>', fitContent: true, embedded: true, allowNewTab: false } })
    try {
      const stage = wrapper.get('.pelican-preview-stage').element
      Object.defineProperties(stage, { clientWidth: { value: 320, configurable: true }, clientHeight: { value: 200, configurable: true } })
      resize(); await flushPromises()
      const frame = wrapper.get('iframe').element as HTMLIFrameElement
      expect(frame.style.transform).toContain('scale(0.25)')
      expect(frame.getAttribute('sandbox')).toBe('allow-scripts')
      expect(frame.getAttribute('scrolling')).toBe('no')
      Object.defineProperties(stage, { clientWidth: { value: 1280 }, clientHeight: { value: 900 } })
      resize(); await flushPromises()
      expect(frame.style.transform).toContain('scale(1)')
      await wrapper.findAll('button').find(b => b.text() === 'pelican.replay')!.trigger('click')
      expect((wrapper.get('iframe').element as HTMLIFrameElement).style.height).toBe('800px')
    } finally { wrapper.unmount(); vi.unstubAllGlobals() }
    expect(disconnect).toHaveBeenCalled()
  })
})
