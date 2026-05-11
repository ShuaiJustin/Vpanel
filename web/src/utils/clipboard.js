export async function copyText(text) {
  if (!text) {
    throw new Error('没有可复制的内容')
  }

  if (
    typeof window !== 'undefined' &&
    window.isSecureContext &&
    typeof navigator !== 'undefined' &&
    navigator.clipboard &&
    typeof navigator.clipboard.writeText === 'function'
  ) {
    try {
      await navigator.clipboard.writeText(text)
      return
    } catch {
      // Fall through to the legacy textarea fallback below.
    }
  }

  if (typeof document === 'undefined' || !document.body) {
    throw new Error('浏览器不支持自动复制，请手动复制')
  }

  const textarea = document.createElement('textarea')
  const selection = document.getSelection()
  const originalRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null
  const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null

  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.top = '0'
  textarea.style.left = '-9999px'
  textarea.style.opacity = '0'

  document.body.appendChild(textarea)

  try {
    textarea.focus()
    textarea.select()
    textarea.setSelectionRange(0, textarea.value.length)

    if (!document.execCommand('copy')) {
      throw new Error('浏览器未允许自动复制，请手动复制')
    }
  } finally {
    document.body.removeChild(textarea)

    if (selection) {
      selection.removeAllRanges()
      if (originalRange) {
        selection.addRange(originalRange)
      }
    }

    if (activeElement && typeof activeElement.focus === 'function') {
      activeElement.focus()
    }
  }
}
