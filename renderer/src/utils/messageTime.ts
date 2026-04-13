import { getLocale, t } from '../i18n'

function formatHoursAndMinutes(date: Date) {
  return new Intl.DateTimeFormat(getLocale(), {
    hour: '2-digit',
    hour12: false,
    minute: '2-digit',
  }).format(date)
}

function isSameLocalDay(a: Date, b: Date) {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  )
}

export function formatMessageTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''

  const now = new Date()
  if (isSameLocalDay(date, now)) {
    return formatHoursAndMinutes(date)
  }

  const yesterday = new Date(now)
  yesterday.setDate(yesterday.getDate() - 1)
  if (isSameLocalDay(date, yesterday)) {
    return t('time.yesterdayAt', { time: formatHoursAndMinutes(date) })
  }
  const locale = getLocale()
  const time = formatHoursAndMinutes(date)
  if (date.getFullYear() === now.getFullYear()) {
    const monthDay = new Intl.DateTimeFormat(locale, {
      day: '2-digit',
      month: '2-digit',
    }).format(date)
    return `${monthDay} ${time}`
  }

  const yearMonthDay = new Intl.DateTimeFormat(locale, {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).format(date)
  return `${yearMonthDay} ${time}`
}
