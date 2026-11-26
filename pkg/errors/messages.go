// Package errors provides structured error types for the V Panel application.
package errors

// 用户友好的错误消息常量
// 这些消息会直接显示给用户，应该清晰、友好、可操作

// 通用错误消息
const (
	MsgInternalError     = "服务器内部错误，请稍后重试"
	MsgDatabaseError     = "数据库操作失败，请稍后重试"
	MsgInvalidRequest    = "请求参数错误，请检查输入"
	MsgUnauthorized      = "未授权访问，请先登录"
	MsgForbidden         = "没有权限执行此操作"
	MsgNotFound          = "请求的资源不存在"
	MsgConflict          = "资源已存在，请检查后重试"
	MsgRateLimitExceeded = "请求过于频繁，请稍后再试"
	MsgServiceUnavailable = "服务暂时不可用，请稍后重试"
)

// 用户相关错误消息
const (
	MsgUserNotFound         = "用户不存在"
	MsgUserAlreadyExists    = "用户名已被使用"
	MsgEmailAlreadyExists   = "邮箱已被注册"
	MsgInvalidCredentials   = "用户名或密码错误"
	MsgUserDisabled         = "账号已被禁用，请联系管理员"
	MsgUserExpired          = "账号已过期，请续费"
	MsgTrafficExceeded      = "流量已用完，请购买流量包"
	MsgPasswordTooWeak      = "密码强度不足，至少需要8个字符"
	MsgPasswordMismatch     = "两次输入的密码不一致"
	MsgOldPasswordIncorrect = "原密码错误"
)

// 代理相关错误消息
const (
	MsgProxyNotFound      = "代理配置不存在"
	MsgProxyPortInUse     = "端口已被占用，请选择其他端口"
	MsgProxyCreateFailed  = "创建代理失败，请检查配置"
	MsgProxyUpdateFailed  = "更新代理失败，请检查配置"
	MsgProxyDeleteFailed  = "删除代理失败"
	MsgProxyStartFailed   = "启动代理失败"
	MsgProxyStopFailed    = "停止代理失败"
	MsgInvalidProtocol    = "不支持的协议类型"
	MsgInvalidProxyConfig = "代理配置无效"
)

// 节点相关错误消息
const (
	MsgNodeNotFound          = "节点不存在"
	MsgNodeAlreadyExists     = "节点名称已存在"
	MsgNodeCreateFailed      = "创建节点失败"
	MsgNodeUpdateFailed      = "更新节点失败"
	MsgNodeDeleteFailed      = "删除节点失败"
	MsgNodeUnhealthy         = "节点状态异常"
	MsgNodeConnectionFailed  = "无法连接到节点"
	MsgNodeDeployFailed      = "节点部署失败"
	MsgNodeSSHConfigInvalid  = "SSH 配置无效"
	MsgNodeAgentNotInstalled = "节点 Agent 未安装"
)

// 证书相关错误消息
const (
	MsgCertNotFound        = "证书不存在"
	MsgCertAlreadyExists   = "该域名的证书已存在"
	MsgCertApplyFailed     = "证书申请失败"
	MsgCertRenewFailed     = "证书续期失败"
	MsgCertExpired         = "证书已过期"
	MsgCertExpiringSoon    = "证书即将过期"
	MsgCertDeployFailed    = "证书部署失败"
	MsgCertInvalidFormat   = "证书格式无效"
	MsgAcmeNotInstalled    = "acme.sh 未安装"
	MsgCertUploadFailed    = "证书上传失败"
)

// 订单和支付相关错误消息
const (
	MsgOrderNotFound       = "订单不存在"
	MsgOrderCreateFailed   = "创建订单失败"
	MsgOrderCancelFailed   = "取消订单失败"
	MsgOrderAlreadyPaid    = "订单已支付"
	MsgOrderExpired        = "订单已过期"
	MsgPaymentFailed       = "支付失败，请重试"
	MsgPaymentMethodInvalid = "不支持的支付方式"
	MsgInsufficientBalance = "余额不足"
	MsgCouponInvalid       = "优惠券无效或已过期"
	MsgCouponUsed          = "优惠券已被使用"
)

// 套餐相关错误消息
const (
	MsgPlanNotFound      = "套餐不存在"
	MsgPlanNotAvailable  = "套餐暂时不可用"
	MsgPlanUpgradeFailed = "套餐升级失败"
	MsgPlanDowngradeFailed = "套餐降级失败"
	MsgTrialAlreadyUsed  = "试用已使用过"
	MsgTrialNotAvailable = "试用暂时不可用"
)

// 认证相关错误消息
const (
	MsgTokenExpired        = "登录已过期，请重新登录"
	MsgTokenInvalid        = "无效的访问令牌"
	MsgRefreshTokenExpired = "刷新令牌已过期，请重新登录"
	Msg2FARequired         = "需要二次验证"
	Msg2FAInvalid          = "二次验证码错误"
	MsgEmailNotVerified    = "邮箱未验证，请先验证邮箱"
	MsgVerificationExpired = "验证链接已过期"
	MsgVerificationInvalid = "验证链接无效"
	MsgAccountLocked       = "账号已被锁定，请30分钟后重试或联系管理员"
	MsgTooManyLoginAttempts = "登录失败次数过多，请稍后再试"
	MsgForcePasswordChange = "需要修改密码才能继续使用"
	MsgSessionConflict     = "账号在其他地方登录，当前会话已失效"
	MsgIPNotAllowed        = "当前 IP 地址不允许登录"
)

// 文件和上传相关错误消息
const (
	MsgFileUploadFailed  = "文件上传失败"
	MsgFileTooLarge      = "文件大小超过限制"
	MsgFileTypeInvalid   = "不支持的文件类型"
	MsgFileNotFound      = "文件不存在"
	MsgFileReadFailed    = "文件读取失败"
	MsgFileWriteFailed   = "文件写入失败"
)

// 配置相关错误消息
const (
	MsgConfigInvalid      = "配置无效"
	MsgConfigUpdateFailed = "配置更新失败"
	MsgConfigNotFound     = "配置不存在"
)

// Xray 相关错误消息
const (
	MsgXrayNotRunning    = "Xray 未运行"
	MsgXrayStartFailed   = "Xray 启动失败"
	MsgXrayStopFailed    = "Xray 停止失败"
	MsgXrayRestartFailed = "Xray 重启失败"
	MsgXrayConfigInvalid = "Xray 配置无效"
	MsgXrayNotInstalled  = "Xray 未安装"
)

// IP 限制相关错误消息
const (
	MsgIPRestricted      = "IP 地址受限"
	MsgIPBlacklisted     = "IP 地址已被封禁"
	MsgTooManyDevices    = "同时在线设备数超过限制"
	MsgDeviceKickFailed  = "踢出设备失败"
)

// 工单相关错误消息
const (
	MsgTicketNotFound     = "工单不存在"
	MsgTicketCreateFailed = "创建工单失败"
	MsgTicketClosed       = "工单已关闭"
	MsgTicketReplyFailed  = "回复工单失败"
)

// 验证相关错误消息
const (
	MsgFieldRequired     = "此字段为必填项"
	MsgFieldTooShort     = "字段长度不足"
	MsgFieldTooLong      = "字段长度超过限制"
	MsgFieldInvalidFormat = "字段格式无效"
	MsgEmailInvalid      = "邮箱格式无效"
	MsgURLInvalid        = "URL 格式无效"
	MsgPortInvalid       = "端口号无效"
	MsgIPInvalid         = "IP 地址格式无效"
	MsgDateInvalid       = "日期格式无效"
	MsgNumberInvalid     = "数字格式无效"
	MsgNumberTooSmall    = "数值过小"
	MsgNumberTooLarge    = "数值过大"
)

// 网络和连接相关错误消息
const (
	MsgNetworkError       = "网络连接失败，请检查网络后重试"
	MsgConnectionTimeout  = "连接超时，请稍后重试"
	MsgServiceMaintenance = "系统维护中，请稍后访问"
)

// 操作反馈相关错误消息
const (
	MsgOperationSuccess    = "操作成功"
	MsgOperationInProgress = "操作正在进行中，请稍候"
	MsgOperationQueued     = "操作已加入队列，稍后执行"
)

// NewUserFriendlyError 创建用户友好的错误
func NewUserFriendlyError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// NewUserNotFoundError 创建用户不存在错误
func NewUserNotFoundError(id any) *AppError {
	return &AppError{
		Code:     ErrCodeNotFound,
		Message:  MsgUserNotFound,
		Entity:   "user",
		EntityID: id,
	}
}

// NewProxyNotFoundError 创建代理不存在错误
func NewProxyNotFoundError(id any) *AppError {
	return &AppError{
		Code:     ErrCodeNotFound,
		Message:  MsgProxyNotFound,
		Entity:   "proxy",
		EntityID: id,
	}
}

// NewNodeNotFoundError 创建节点不存在错误
func NewNodeNotFoundError(id any) *AppError {
	return &AppError{
		Code:     ErrCodeNotFound,
		Message:  MsgNodeNotFound,
		Entity:   "node",
		EntityID: id,
	}
}

// NewCertNotFoundError 创建证书不存在错误
func NewCertNotFoundError(id any) *AppError {
	return &AppError{
		Code:     ErrCodeNotFound,
		Message:  MsgCertNotFound,
		Entity:   "certificate",
		EntityID: id,
	}
}

// NewOrderNotFoundError 创建订单不存在错误
func NewOrderNotFoundError(id any) *AppError {
	return &AppError{
		Code:     ErrCodeNotFound,
		Message:  MsgOrderNotFound,
		Entity:   "order",
		EntityID: id,
	}
}

// NewInvalidCredentialsError 创建凭证无效错误
func NewInvalidCredentialsError() *AppError {
	return &AppError{
		Code:    ErrCodeUnauthorized,
		Message: MsgInvalidCredentials,
	}
}

// NewTokenExpiredError 创建令牌过期错误
func NewTokenExpiredError() *AppError {
	return &AppError{
		Code:    ErrCodeUnauthorized,
		Message: MsgTokenExpired,
	}
}

// NewInsufficientBalanceError 创建余额不足错误
func NewInsufficientBalanceError() *AppError {
	return &AppError{
		Code:    ErrCodeBadRequest,
		Message: MsgInsufficientBalance,
	}
}
