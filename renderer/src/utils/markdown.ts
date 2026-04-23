import hljs from 'highlight.js/lib/common'
import { Marked } from 'marked'
import { chipHtml, type TokenKind } from './mentions'

const ALLOWED_TAGS = new Set([
  'a',
  'blockquote',
  'br',
  'code',
  'del',
  'em',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'hr',
  'img',
  'input',
  'li',
  'ol',
  'p',
  'pre',
  'span',
  'strong',
  'table',
  'tbody',
  'td',
  'th',
  'thead',
  'tr',
  'ul',
])

const BLOCKED_TAGS = new Set([
  'embed',
  'iframe',
  'link',
  'meta',
  'object',
  'script',
  'style',
])

const ALLOWED_ATTRIBUTES: Record<string, Set<string>> = {
  a: new Set(['href', 'title']),
  code: new Set(['class']),
  img: new Set(['src', 'alt', 'title']),
  input: new Set(['type', 'checked', 'disabled']),
  ol: new Set(['start']),
  span: new Set(['class', 'data-kind', 'data-value', 'data-label']),
  td: new Set(['align']),
  th: new Set(['align']),
}

const SAFE_LINK_PROTOCOLS = new Set(['http:', 'https:', 'mailto:', 'tel:'])
const SAFE_IMAGE_PROTOCOLS = new Set(['http:', 'https:'])
const markdown = new Marked({
  async: false,
  breaks: true,
  gfm: true,
})

// Treat inline tokens (@[Name](agent:<ID>), /[Name](skill:<Name>)) as
// first-class inline tokens, rendered directly as chips. Keeps the markdown
// pipeline clean and gives tokens the same lifecycle as other inline nodes.
markdown.use({
  extensions: [
    {
      name: 'inlineToken',
      level: 'inline',
      start(src: string) {
        const a = src.indexOf('@[')
        const s = src.indexOf('/[')
        if (a === -1) return s === -1 ? undefined : s
        if (s === -1) return a
        return Math.min(a, s)
      },
      tokenizer(src: string) {
        const m = /^([@/])\[([^\]]+)\]\((agent|skill):([^)]+)\)/.exec(src)
        if (!m) return undefined
        const trigger = m[1]
        const kind = m[3] as TokenKind
        // Reject mismatched trigger/kind pairs so '/[x](agent:y)' doesn't
        // sneak through as a valid chip.
        if ((trigger === '@') !== (kind === 'agent')) return undefined
        return {
          type: 'inlineToken',
          raw: m[0],
          kind,
          value: m[4],
          label: m[2],
        }
      },
      renderer(token: { kind: TokenKind; value: string; label: string }) {
        return chipHtml(token.kind, token.value, token.label)
      },
    },
  ],
})

export function renderMarkdown(source: string): string {
  if (!source) return ''

  const rawHtml = markdown.parse(source)
  if (typeof rawHtml !== 'string') return ''
  if (typeof DOMParser === 'undefined') {
    return escapeHtml(source)
  }

  return sanitizeMarkdownHtml(rawHtml)
}

function sanitizeMarkdownHtml(html: string): string {
  const parser = new DOMParser()
  const doc = parser.parseFromString(html, 'text/html')
  sanitizeTree(doc.body)
  highlightCodeBlocks(doc.body)
  return doc.body.innerHTML
}

function highlightCodeBlocks(root: ParentNode) {
  for (const node of root.querySelectorAll('pre > code')) {
    const source = node.textContent ?? ''
    if (!source.trim()) continue

    const requestedLanguage = extractLanguage(node)
    const language = normalizeLanguage(requestedLanguage)

    try {
      const result = language && hljs.getLanguage(language)
        ? hljs.highlight(source, { language, ignoreIllegals: true })
        : hljs.highlightAuto(source)

      node.innerHTML = result.value
      node.classList.add('hljs')
      if (requestedLanguage) {
        node.classList.add(`language-${requestedLanguage}`)
      } else if (result.language) {
        node.classList.add(`language-${result.language}`)
      }
    } catch {
      node.textContent = source
    }
  }
}

function sanitizeTree(parent: ParentNode) {
  for (const node of Array.from(parent.childNodes)) {
    if (node.nodeType === Node.COMMENT_NODE) {
      node.remove()
      continue
    }

    if (node.nodeType !== Node.ELEMENT_NODE) continue

    const element = node as Element
    sanitizeTree(element)
    sanitizeElement(element)
  }
}

function sanitizeElement(element: Element) {
  const tag = element.tagName.toLowerCase()

  if (BLOCKED_TAGS.has(tag)) {
    element.remove()
    return
  }

  if (!ALLOWED_TAGS.has(tag)) {
    unwrapElement(element)
    return
  }

  sanitizeAttributes(element, tag)

  if (tag === 'a') {
    const safeHref = sanitizeUrl(element.getAttribute('href'), 'link')
    if (!safeHref) {
      unwrapElement(element)
      return
    }
    element.setAttribute('href', safeHref)
    if (!safeHref.startsWith('#')) {
      element.setAttribute('target', '_blank')
      element.setAttribute('rel', 'noreferrer noopener')
    }
  }

  if (tag === 'img') {
    const safeSrc = sanitizeUrl(element.getAttribute('src'), 'image')
    if (!safeSrc) {
      element.remove()
      return
    }
    element.setAttribute('src', safeSrc)
    element.setAttribute('loading', 'lazy')
  }

  if (tag === 'code') {
    const className = element.getAttribute('class')
    if (
      className &&
      !className
        .split(/\s+/)
        .filter(Boolean)
        .every((part) => /^language-[A-Za-z0-9._+-]+$/.test(part))
    ) {
      element.removeAttribute('class')
    }
  }

  if (tag === 'input') {
    if (element.getAttribute('type') !== 'checkbox') {
      element.remove()
      return
    }
    element.setAttribute('disabled', '')
  }

  // Only permit <span> in its mention-chip shape; any other span is
  // unwrapped so raw LLM HTML can't sneak styled text through.
  if (tag === 'span') {
    if (
      element.getAttribute('class')?.trim() !== 'mention-chip' ||
      !element.hasAttribute('data-agent-id')
    ) {
      unwrapElement(element)
      return
    }
  }

  if (tag === 'li' && element.querySelector(':scope > input[type="checkbox"]')) {
    element.classList.add('task-list-item')
  }
  if ((tag === 'ul' || tag === 'ol') && element.querySelector(':scope > li.task-list-item')) {
    element.classList.add('task-list')
  }
}

function sanitizeAttributes(element: Element, tag: string) {
  const allowed = ALLOWED_ATTRIBUTES[tag] ?? new Set<string>()

  for (const attr of Array.from(element.attributes)) {
    const name = attr.name.toLowerCase()

    if (name.startsWith('on')) {
      element.removeAttribute(attr.name)
      continue
    }

    if (!allowed.has(name)) {
      element.removeAttribute(attr.name)
    }
  }
}

function sanitizeUrl(value: string | null, kind: 'link' | 'image'): string | null {
  if (!value) return null

  const trimmed = value.trim()
  if (!trimmed) return null
  if (trimmed.startsWith('#')) return trimmed
  if (trimmed.startsWith('//')) return null

  const hasScheme = /^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(trimmed)
  if (!hasScheme) return trimmed

  try {
    const parsed = new URL(trimmed)
    const allowedProtocols = kind === 'image' ? SAFE_IMAGE_PROTOCOLS : SAFE_LINK_PROTOCOLS
    return allowedProtocols.has(parsed.protocol) ? parsed.toString() : null
  } catch {
    return null
  }
}

function extractLanguage(element: Element): string {
  const className = element.getAttribute('class') ?? ''
  for (const part of className.split(/\s+/).filter(Boolean)) {
    if (part.startsWith('language-')) {
      return part.slice('language-'.length).toLowerCase()
    }
  }
  return ''
}

function normalizeLanguage(language: string) {
  const normalized = language.toLowerCase()
  switch (normalized) {
    case 'c++':
      return 'cpp'
    case 'c#':
      return 'csharp'
    case 'js':
      return 'javascript'
    case 'ts':
      return 'typescript'
    case 'sh':
    case 'shell':
      return 'bash'
    case 'yml':
      return 'yaml'
    default:
      return normalized
  }
}

function unwrapElement(element: Element) {
  element.replaceWith(...Array.from(element.childNodes))
}

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}
