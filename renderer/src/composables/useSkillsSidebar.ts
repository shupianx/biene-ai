import { ref } from 'vue'
import { getDesktopBridge } from '../runtime'

const skillsSidebarOpen = ref(false)
const skillsSidebarWidth = ref(280)

export function useSkillsSidebar() {
  async function syncDesktopSidebar(open: boolean) {
    const bridge = getDesktopBridge()
    if (!bridge?.isElectron || bridge.windowKind !== 'main' || !bridge.setSkillsSidebarOpen) {
      return
    }
    await bridge.setSkillsSidebarOpen(open, skillsSidebarWidth.value)
  }

  async function openSkillsSidebar() {
    if (skillsSidebarOpen.value) return
    await syncDesktopSidebar(true)
    skillsSidebarOpen.value = true
  }

  async function closeSkillsSidebar() {
    if (!skillsSidebarOpen.value) return
    skillsSidebarOpen.value = false
    await syncDesktopSidebar(false)
  }

  async function toggleSkillsSidebar() {
    if (skillsSidebarOpen.value) {
      await closeSkillsSidebar()
      return
    }
    await openSkillsSidebar()
  }

  return {
    skillsSidebarOpen,
    skillsSidebarWidth,
    openSkillsSidebar,
    closeSkillsSidebar,
    toggleSkillsSidebar,
  }
}
