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
  while (isAgentNameTaken(`Agent ${index}`, existingNames)) {
    index += 1
  }
  return `Agent ${index}`
}
