const BLOCKED_TAGS = new Set([
  'SCRIPT',
  'STYLE',
  'IFRAME',
  'OBJECT',
  'EMBED',
  'LINK',
  'META',
  'BASE',
  'FORM',
])

const URL_ATTRS = new Set(['href', 'src', 'xlink:href', 'action', 'formaction'])
const SAFE_PROTOCOLS = new Set(['http:', 'https:', 'mailto:', 'tel:'])

function hasExplicitProtocol(value) {
  return /^[a-z][a-z0-9+.-]*:/i.test(value)
}

function isUnsafeUrl(value) {
  const normalized = String(value || '').trim()
  if (!normalized) {
    return false
  }

  if (
    normalized.startsWith('#') ||
    normalized.startsWith('/') ||
    normalized.startsWith('./') ||
    normalized.startsWith('../')
  ) {
    return false
  }

  if (/^(javascript|vbscript|data):/i.test(normalized)) {
    return true
  }

  if (!hasExplicitProtocol(normalized)) {
    return false
  }

  try {
    const parsed = new URL(normalized)
    return !SAFE_PROTOCOLS.has(parsed.protocol)
  } catch {
    return true
  }
}

function sanitizeElement(element) {
  if (BLOCKED_TAGS.has(element.tagName)) {
    element.remove()
    return
  }

  for (const attr of [...element.attributes]) {
    const attrName = attr.name.toLowerCase()

    if (attrName.startsWith('on')) {
      element.removeAttribute(attr.name)
      continue
    }

    if (URL_ATTRS.has(attrName) && isUnsafeUrl(attr.value)) {
      element.removeAttribute(attr.name)
    }
  }

  if (element.tagName === 'A' && element.getAttribute('target') === '_blank') {
    element.setAttribute('rel', 'noopener noreferrer')
  }

  for (const child of [...element.children]) {
    sanitizeElement(child)
  }
}

export function sanitizeHtml(rawHtml) {
  if (!rawHtml) {
    return ''
  }

  if (typeof DOMParser === 'undefined') {
    return String(rawHtml)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;')
  }

  const parser = new DOMParser()
  const doc = parser.parseFromString(String(rawHtml), 'text/html')

  for (const child of [...doc.body.children]) {
    sanitizeElement(child)
  }

  return doc.body.innerHTML
}

export default sanitizeHtml
