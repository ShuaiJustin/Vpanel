import { computed, onScopeDispose, reactive, ref, shallowRef, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useNodeStore } from '@/stores/node'
import { copyText } from '@/utils/clipboard'
import { extractErrorMessage } from '@/utils/entitlement'

export function useNodeTokenDialog() {
  const nodeStore = useNodeStore()
  const tokenDialogVisible = ref(false)
  const session = shallowRef(null)
  const pendingNodeIds = reactive(new Set())
  const tokenNode = computed(() => session.value?.node)
  const hasToken = computed(() => {
    const target = tokenNode.value
    if (!target) return undefined
    const latest = nodeStore.currentNode?.id === target.id
      ? nodeStore.currentNode
      : nodeStore.nodes.find(node => node.id === target.id)
    return (latest || target).has_token
  })
  const tokenLoading = computed(() => pendingNodeIds.has(tokenNode.value?.id))
  const currentToken = ref('')
  const showToken = ref(false)

  const clearToken = () => {
    currentToken.value = ''
    showToken.value = false
  }

  const closeTokenDialog = () => {
    session.value = null
    tokenDialogVisible.value = false
    clearToken()
  }

  watch(tokenDialogVisible, (visible) => {
    if (!visible) closeTokenDialog()
  }, { flush: 'sync' })
  onScopeDispose(closeTokenDialog)

  const openTokenDialog = (node) => {
    if (!node?.id) return
    session.value = { node }
    clearToken()
    tokenDialogVisible.value = true
  }

  const runTokenAction = async ({ action, warning, request, success }) => {
    const activeSession = session.value
    const nodeId = activeSession?.node.id
    if (!nodeId || pendingNodeIds.has(nodeId)) return

    pendingNodeIds.add(nodeId)
    try {
      try {
        await ElMessageBox.confirm(warning, `${action} Token`, { type: 'warning' })
      } catch {
        return
      }
      if (session.value !== activeSession) return

      const result = await request(nodeId)
      if (session.value !== activeSession) return

      currentToken.value = result?.token || ''
      showToken.value = false
      ElMessage.success(success)
    } catch (error) {
      if (session.value === activeSession) {
        ElMessage.error(extractErrorMessage(error) || `${action} Token 失败`)
      }
    } finally {
      pendingNodeIds.delete(nodeId)
    }
  }

  const handleGenerateToken = () => runTokenAction({
    action: '生成',
    warning: '生成新 Token 会使旧 Token 立即失效，已部署的 Agent 需使用新 Token 重新连接。确定继续？',
    request: nodeStore.generateToken,
    success: 'Token 生成成功'
  })

  const handleRotateToken = () => runTokenAction({
    action: '轮换',
    warning: '轮换 Token 后，旧 Token 将立即失效，确定继续？',
    request: nodeStore.rotateToken,
    success: 'Token 轮换成功'
  })

  const handleRevokeToken = () => runTokenAction({
    action: '撤销',
    warning: '撤销 Token 后，节点将无法连接，确定继续？',
    request: nodeStore.revokeToken,
    success: 'Token 已撤销'
  })

  const copyToken = async () => {
    try {
      await copyText(currentToken.value)
      ElMessage.success('已复制到剪贴板')
    } catch (error) {
      ElMessage.error(extractErrorMessage(error) || '复制失败')
    }
  }

  return {
    tokenDialogVisible, tokenNode, tokenLoading, currentToken, showToken, hasToken,
    openTokenDialog, closeTokenDialog, handleGenerateToken, handleRotateToken, handleRevokeToken, copyToken
  }
}
