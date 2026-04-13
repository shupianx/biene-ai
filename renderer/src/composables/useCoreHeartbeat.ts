import { onBeforeUnmount, onMounted, ref } from 'vue'
import { buildCoreHeaders, buildCoreUrl, getCoreBaseUrl, getDesktopBridge } from '../runtime'

export function useCoreHeartbeat(intervalMs = 2500) {
  const isCoreHealthy = ref(Boolean(getCoreBaseUrl()))
  let timer: number | null = null
  let detach: (() => void) | null = null

  async function ping() {
    const baseUrl = getCoreBaseUrl()
    if (!baseUrl) {
      isCoreHealthy.value = false
      return
    }

    const controller = new AbortController()
    const timeout = window.setTimeout(() => controller.abort(), 1200)

    try {
      const response = await fetch(buildCoreUrl('/api/health'), {
        cache: 'no-store',
        headers: buildCoreHeaders(),
        signal: controller.signal,
      })
      isCoreHealthy.value = response.ok
    } catch {
      isCoreHealthy.value = false
    } finally {
      window.clearTimeout(timeout)
    }
  }

  const bridge = getDesktopBridge()

  if (bridge?.isElectron && bridge.getCoreStatus) {
    onMounted(() => {
      void bridge.getCoreStatus().then((status) => {
        isCoreHealthy.value = status.healthy
      }).catch(() => {
        isCoreHealthy.value = false
      })

      const onCoreStatus = (event: Event) => {
        const detail = (event as CustomEvent<{ healthy?: boolean }>).detail
        isCoreHealthy.value = Boolean(detail?.healthy)
      }

      window.addEventListener('biene:core-status', onCoreStatus as EventListener)
      detach = () => window.removeEventListener('biene:core-status', onCoreStatus as EventListener)
    })

    onBeforeUnmount(() => {
      detach?.()
    })

    return {
      isCoreHealthy,
      pingCore: async () => {
        try {
          isCoreHealthy.value = (await bridge.getCoreStatus()).healthy
        } catch {
          isCoreHealthy.value = false
        }
      },
    }
  }

  onMounted(() => {
    void ping()
    timer = window.setInterval(() => {
      void ping()
    }, intervalMs)
  })

  onBeforeUnmount(() => {
    if (timer != null) {
      window.clearInterval(timer)
    }
  })

  return {
    isCoreHealthy,
    pingCore: ping,
  }
}
