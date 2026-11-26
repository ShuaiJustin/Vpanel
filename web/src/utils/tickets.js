export const ticketStatuses = {
  open: { label: '待处理', type: 'warning' },
  waiting: { label: '等待客服回复', type: 'warning' },
  answered: { label: '客服已回复', type: 'success' },
  pending: { label: '处理中', type: 'primary' },
  resolved: { label: '已解决', type: 'success' },
  closed: { label: '已关闭', type: 'info' }
}

export const ticketPriorities = {
  low: { label: '低', type: 'info' },
  normal: { label: '普通', type: 'info' },
  medium: { label: '普通', type: 'info' },
  high: { label: '高', type: 'warning' },
  urgent: { label: '紧急', type: 'danger' }
}

export const ticketCategories = {
  account: '账户问题',
  connection: '连接问题',
  subscription: '订阅问题',
  payment: '支付问题',
  suggestion: '功能建议',
  other: '其他问题',
  technical: '技术问题',
  billing: '账单问题'
}
