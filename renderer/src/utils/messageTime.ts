function pad(value: number) {
  return String(value).padStart(2, '0')
}

function formatHoursAndMinutes(date: Date) {
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
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
    return `昨天 ${formatHoursAndMinutes(date)}`
  }

  const monthDay = `${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
  const time = formatHoursAndMinutes(date)
  if (date.getFullYear() === now.getFullYear()) {
    return `${monthDay} ${time}`
  }

  return `${date.getFullYear()}-${monthDay} ${time}`
}
