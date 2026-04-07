import { useRouter } from 'vue-router'
import { getDesktopBridge } from '../runtime'

export function useAgentNavigation() {
  const router = useRouter()

  async function openAgent(agentId: string) {
    const bridge = getDesktopBridge()
    if (bridge?.openAgentWindow) {
      await bridge.openAgentWindow(agentId)
      return
    }
    await router.push(`/agent/${encodeURIComponent(agentId)}`)
  }

  async function closeAgentView() {
    if (getDesktopBridge()?.isElectron) {
      window.close()
      return
    }
    await router.push('/')
  }

  return {
    openAgent,
    closeAgentView,
  }
}
