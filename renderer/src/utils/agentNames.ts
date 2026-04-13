import { t } from '../i18n'

export function normalizeAgentName(name: string) {
  return name.trim().toLowerCase()
}

export function isAgentNameTaken(name: string, existingNames: string[]) {
  const normalized = normalizeAgentName(name)
  if (!normalized) return false
  return existingNames.some((item) => normalizeAgentName(item) === normalized)
}

export function nextDefaultAgentName(existingNames: string[]) {
  let index = existingNames.length + 1
  while (isAgentNameTaken(t('agentName.defaultName', { index }), existingNames)) {
    index += 1
  }
  return t('agentName.defaultName', { index })
}
