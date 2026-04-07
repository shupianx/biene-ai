import type { SessionPermissions } from '../api/http'

export type PermissionKey = keyof SessionPermissions

export interface PermissionDefinition {
  key: PermissionKey
  label: string
  description: string
}

export const permissionDefinitions: PermissionDefinition[] = [
  {
    key: 'write',
    label: 'File changes',
    description: 'Allows both Write and Edit tool calls.',
  },
  {
    key: 'send_to_agent',
    label: 'Agent transfer',
    description: 'Allows sending messages or files to other agents.',
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

export function getPermissionLabel(key: string) {
  return permissionDefinitions.find((item) => item.key === key)?.label ?? key
}

export function getPermissionDescription(key: string) {
  return permissionDefinitions.find((item) => item.key === key)?.description ?? ''
}
