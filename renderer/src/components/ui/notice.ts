import type { Component } from 'vue'

export type NoticeTone = 'warn' | 'ok' | 'err' | 'info'

export type Notice = {
  id: string
  tone: NoticeTone
  icon?: Component
  text?: string
  body?: Component
  bodyProps?: Record<string, unknown>
  ttlMs?: number
}

let seq = 0
export function createNoticeId(prefix = 'n'): string {
  seq = (seq + 1) | 0
  return `${prefix}:${Date.now().toString(36)}:${seq.toString(36)}`
}
