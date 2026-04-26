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
    key: 'execute',
    labelKey: 'permissions.execute.label',
    descriptionKey: 'permissions.execute.description',
  },
  {
    key: 'write',
    labelKey: 'permissions.write.label',
    descriptionKey: 'permissions.write.description',
  },
  {
    key: 'send_message_to_agent',
    labelKey: 'permissions.send_message_to_agent.label',
    descriptionKey: 'permissions.send_message_to_agent.description',
  },
  {
    key: 'cowork',
    labelKey: 'permissions.cowork.label',
    descriptionKey: 'permissions.cowork.description',
  },
]

export function defaultPermissions(): SessionPermissions {
  return {
    execute: false,
    write: false,
    send_message_to_agent: false,
    cowork: false,
  }
}

export function clonePermissions(permissions: SessionPermissions): SessionPermissions {
  return {
    execute: permissions.execute,
    write: permissions.write,
    send_message_to_agent: permissions.send_message_to_agent,
    cowork: permissions.cowork,
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
