import type { SessionPermissions } from '../api/http'
import { t } from '../i18n'

export type PermissionKey = keyof SessionPermissions

export interface PermissionDefinition {
  key: PermissionKey
  labelKey: string
  descriptionKey: string
}

export const permissionDefinitions: PermissionDefinition[] = [
  {
    key: 'write',
    labelKey: 'permissions.write.label',
    descriptionKey: 'permissions.write.description',
  },
  {
    key: 'send_to_agent',
    labelKey: 'permissions.send_to_agent.label',
    descriptionKey: 'permissions.send_to_agent.description',
  },
]

export function defaultPermissions(): SessionPermissions {
  return {
    write: false,
    send_to_agent: false,
  }
}

export function clonePermissions(permissions: SessionPermissions): SessionPermissions {
  return {
    write: permissions.write,
    send_to_agent: permissions.send_to_agent,
  }
}

export function listPermissionDefinitions() {
  return permissionDefinitions.map((item) => ({
    key: item.key,
    label: t(item.labelKey),
    description: t(item.descriptionKey),
  }))
}

export function getPermissionLabel(key: string) {
  const item = permissionDefinitions.find((definition) => definition.key === key)
  return item ? t(item.labelKey) : key
}

export function getPermissionDescription(key: string) {
  const item = permissionDefinitions.find((definition) => definition.key === key)
  return item ? t(item.descriptionKey) : ''
}
