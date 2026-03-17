export async function copyText(text) {
  if (!text) {
    throw new Error('No text to copy')
  }

  if (
    typeof window !== 'undefined' &&
    window.isSecureContext &&
    typeof navigator !== 'undefined' &&
    navigator.clipboard &&
    typeof navigator.clipboard.writeText === 'function'
  ) {
    await navigator.clipboard.writeText(text)
    return
  }

  if (typeof document === 'undefined' || !document.body) {
    throw new Error('Clipboard is unavailable')
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
      throw new Error('document.execCommand(copy) returned false')
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
