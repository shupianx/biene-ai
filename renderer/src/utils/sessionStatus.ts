import { t } from '../i18n'

export type SessionStatusTone = 'idle' | 'running' | 'error' | 'approval'

export function getSessionStatusTone(session: {
  meta: { status: 'idle' | 'running' | 'error' }
  pendingPermission: unknown
}): SessionStatusTone {
  if (session.meta.status === 'error') return 'error'
  if (session.pendingPermission) return 'approval'
  return session.meta.status
}

export function getSessionStatusLabel(tone: SessionStatusTone) {
  return t(`sessionStatus.${tone}`)
}
