<template>
  <div class="node-form-page">
    <section class="hero-panel">
      <div class="hero-panel__nav">
        <el-button link class="back-link" @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回节点列表
        </el-button>
        <div class="hero-panel__copy">
          <span class="hero-panel__eyebrow">Node Workspace</span>
          <h1 class="page-title">
            {{ pageTitle }}
          </h1>
          <p class="page-subtitle">
            {{ pageDescription }}
          </p>
        </div>
      </div>

      <div class="hero-panel__stats">
        <div class="hero-stat">
          <span class="hero-stat__label">命名模式</span>
          <strong class="hero-stat__value">{{ namingModeLabel }}</strong>
        </div>
        <div class="hero-stat">
          <span class="hero-stat__label">节点地区</span>
          <strong class="hero-stat__value">{{
            form.region || "待识别"
          }}</strong>
        </div>
        <div class="hero-stat">
          <span class="hero-stat__label">部署方式</span>
          <strong class="hero-stat__value">{{ deploymentModeLabel }}</strong>
        </div>
      </div>
    </section>

    <el-form
      ref="formRef"
      v-loading="loading"
      :model="form"
      :rules="rules"
      label-position="top"
      size="large"
      class="node-form"
    >
      <div class="form-layout">
        <div class="form-main">
          <el-card class="section-card" shadow="never">
            <template #header>
              <div class="section-head">
                <div>
                  <h2 class="section-title">基础接入</h2>
                  <p class="section-subtitle">
                    先确定节点身份、地址与负载策略，系统会根据地址自动给出推荐名称。
                  </p>
                </div>
              </div>
            </template>

            <div class="form-grid">
              <el-form-item
                class="field-span-2"
                label="节点地址"
                prop="address"
              >
                <div class="address-field">
                  <div class="address-field__controls">
                    <el-radio-group
                      v-model="addressType"
                      class="address-type-switch"
                    >
                      <el-radio-button label="ip"> IP </el-radio-button>
                      <el-radio-button label="domain"> 域名 </el-radio-button>
                    </el-radio-group>

                    <el-input
                      v-model="form.address"
                      class="address-field__input"
                      :placeholder="
                        addressType === 'ip'
                          ? '输入 IPv4 或 IPv6 地址'
                          : '输入节点域名'
                      "
                      @blur="handleAddressBlur"
                    />
                  </div>

                  <div class="address-field__badges">
                    <span
                      class="address-badge"
                      :class="{ 'is-active': addressType === 'ip' }"
                    >
                      IPv4 / IPv6
                    </span>
                    <span
                      class="address-badge"
                      :class="{ 'is-active': addressType === 'domain' }"
                    >
                      DNS 域名
                    </span>
                    <span class="address-badge">地区识别</span>
                    <span class="address-badge">智能命名</span>
                  </div>
                </div>
                <div class="field-tip">
                  {{
                    addressType === "ip"
                      ? "支持 IPv4、IPv6。输入后会自动识别地区并回填推荐名称。"
                      : "输入域名后会自动解析地址、识别地区并生成推荐名称。"
                  }}
                </div>
              </el-form-item>
              <el-form-item class="field-span-2" label="节点名称" prop="name">
                <div class="name-field">
                  <el-input
                    v-model="form.name"
                    placeholder="请输入节点名称"
                    @input="handleNameInput"
                  />
                  <el-button
                    type="primary"
                    plain
                    class="name-field__action"
                    :loading="suggestionLoading"
                    @click="generateSuggestedName(true)"
                  >
                    自动生成
                  </el-button>
                </div>
                <div :class="['suggestion-banner', suggestionTone]">
                  <span class="suggestion-banner__label">{{
                    suggestionHeadline
                  }}</span>
                  <span class="suggestion-banner__text">{{
                    nameSuggestionMessage
                  }}</span>
                </div>
              </el-form-item>

              <el-form-item label="Agent 端口" prop="port">
                <el-input-number
                  v-model="form.port"
                  class="full-width-input"
                  :min="1"
                  :max="65535"
                />
                <div class="field-tip">
                  Node Agent
                  默认监听端口；修改后需与面板记录和防火墙放行保持一致。
                </div>
              </el-form-item>

              <el-form-item label="地区" prop="region">
                <el-select
                  v-model="form.region"
                  filterable
                  allow-create
                  class="full-width-control"
                  placeholder="选择或输入地区"
                >
                  <el-option label="香港" value="香港" />
                  <el-option label="日本" value="日本" />
                  <el-option label="新加坡" value="新加坡" />
                  <el-option label="美国" value="美国" />
                  <el-option label="韩国" value="韩国" />
                  <el-option label="台湾" value="台湾" />
                  <el-option label="德国" value="德国" />
                  <el-option label="英国" value="英国" />
                </el-select>
                <div class="field-tip">
                  如需固定地区标签，可手动指定，系统会优先遵循你的选择。
                </div>
              </el-form-item>

              <el-form-item
                class="field-span-2"
                label="负载均衡权重"
                prop="weight"
              >
                <div class="slider-field">
                  <el-slider
                    v-model="form.weight"
                    class="slider-field__control"
                    :min="1"
                    :max="100"
                  />
                  <span class="slider-pill">权重 {{ form.weight }}</span>
                </div>
                <div class="field-tip">权重越高，分配到该节点的用户越多。</div>
              </el-form-item>

              <el-form-item label="最大用户数" prop="max_users">
                <el-input-number
                  v-model="form.max_users"
                  class="full-width-input"
                  :min="0"
                />
                <div class="field-tip">0 表示无限制。</div>
              </el-form-item>

              <el-form-item label="所属分组">
                <el-select
                  v-model="form.group_ids"
                  multiple
                  filterable
                  class="full-width-control"
                  placeholder="选择分组（可多选）"
                >
                  <el-option
                    v-for="group in groups"
                    :key="group.id"
                    :label="group.name"
                    :value="group.id"
                  >
                    <span>{{ group.name }}</span>
                    <span class="option-secondary">
                      {{ group.region }}
                    </span>
                  </el-option>
                </el-select>
                <div class="field-tip">节点可以同时属于多个分组。</div>
              </el-form-item>

              <el-form-item class="field-span-2" label="标签">
                <div class="tags-input">
                  <el-tag
                    v-for="(tag, index) in form.tags"
                    :key="index"
                    closable
                    class="tag-chip"
                    @close="removeTag(index)"
                  >
                    {{ tag }}
                  </el-tag>
                  <el-input
                    v-if="showTagInput"
                    ref="tagInputRef"
                    v-model="newTag"
                    size="small"
                    class="tag-editor"
                    @keyup.enter="addTag"
                    @blur="addTag"
                  />
                  <el-button
                    v-else
                    size="small"
                    plain
                    @click="showTagInputField"
                  >
                    + 添加标签
                  </el-button>
                </div>
              </el-form-item>
            </div>
          </el-card>

          <el-card class="section-card" shadow="never">
            <template #header>
              <div class="section-head">
                <div>
                  <h2 class="section-title">节点流量策略</h2>
                  <p class="section-subtitle">
                    控制节点月流量上限、停新分配阈值，并查看当前周期的额度使用情况。
                  </p>
                </div>
              </div>
            </template>

            <div class="form-grid">
              <el-form-item label="月流量上限">
                <div class="quota-field">
                  <el-input-number
                    v-model="form.traffic_limit_value"
                    class="quota-field__input"
                    :min="0"
                    :precision="form.traffic_limit_unit === 'TiB' ? 2 : 0"
                    :step="form.traffic_limit_unit === 'TiB' ? 0.1 : 10"
                  />
                  <el-select
                    v-model="form.traffic_limit_unit"
                    class="quota-field__unit"
                  >
                    <el-option
                      v-for="option in trafficLimitUnitOptions"
                      :key="option.value"
                      :label="option.label"
                      :value="option.value"
                    />
                  </el-select>
                </div>
                <div class="field-tip">
                  0
                  表示不限流量；达到阈值后停止分配新用户，达到上限后节点自动停用，并按月自动重置。
                </div>
              </el-form-item>

              <el-form-item label="停新分配阈值">
                <div class="slider-field">
                  <el-slider
                    v-model="form.alert_traffic_threshold"
                    class="slider-field__control"
                    :min="1"
                    :max="100"
                    :disabled="!hasPreviewTrafficLimit"
                  />
                  <span class="slider-pill">{{
                    hasPreviewTrafficLimit
                      ? `${form.alert_traffic_threshold}%`
                      : "未启用"
                  }}</span>
                </div>
                <div class="field-tip">
                  到达该比例后，节点保留现有用户可用，但不再参与新用户分配。
                </div>
              </el-form-item>

              <el-form-item label="当前策略预览">
                <div class="quota-panel">
                  <div class="quota-panel__headline">
                    {{ trafficQuotaSummary }}
                  </div>
                  <div class="quota-panel__meta">
                    <span>{{ trafficQuotaStateText }}</span>
                    <span v-if="hasPreviewTrafficLimit"
                      >剩余 {{ trafficQuotaRemaining }}</span
                    >
                  </div>
                  <el-progress
                    v-if="hasPreviewTrafficLimit"
                    :percentage="trafficQuotaPercent"
                    :stroke-width="8"
                    :status="trafficQuotaProgressStatus"
                    :show-text="false"
                  />
                  <div class="field-tip-inline">
                    {{
                      isEdit
                        ? `当前节点下次重置：${currentTrafficResetLabel}`
                        : "保存后会自动以当前时间作为首个计费周期锚点。"
                    }}
                  </div>
                </div>
              </el-form-item>

              <el-form-item label="保护机制说明">
                <div class="quota-policy-list">
                  <div class="quota-policy-item">
                    <strong>软阈值</strong>
                    <span
                      >达到阈值后停止新分配，避免节点在月底被迅速打满。</span
                    >
                  </div>
                  <div class="quota-policy-item">
                    <strong>硬上限</strong>
                    <span
                      >达到月上限后节点转为停用，订阅和直连节点都会失效。</span
                    >
                  </div>
                  <div class="quota-policy-item">
                    <strong>月重置</strong>
                    <span
                      >系统每小时检查一次，到期后自动清零并恢复下一计费周期。</span
                    >
                  </div>
                </div>
              </el-form-item>
            </div>
          </el-card>

          <el-card class="section-card" shadow="never">
            <template #header>
              <div class="section-head">
                <div>
                  <h2 class="section-title">接入限制与 TLS</h2>
                  <p class="section-subtitle">
                    配置访问范围、TLS 域名与系统证书，保持节点接入信息完整可读。
                  </p>
                </div>
              </div>
            </template>

            <div class="form-grid">
              <el-form-item class="field-span-2" label="IP 白名单">
                <el-input
                  v-model="form.ip_whitelist_str"
                  type="textarea"
                  :rows="5"
                  placeholder="每行一个 IP 地址，留空表示不限制&#10;支持 CIDR 格式，如 192.168.1.0/24"
                />
                <div class="field-tip">限制可以连接到此节点的 IP 地址。</div>
              </el-form-item>

              <el-form-item label="启用 TLS">
                <div class="switch-field">
                  <el-switch
                    v-model="form.tls_enabled"
                    active-text="开启"
                    inactive-text="关闭"
                  />
                  <div class="field-tip-inline">
                    开启后建议同时配置 TLS 域名并关联系统证书。
                  </div>
                </div>
              </el-form-item>

              <el-form-item label="TLS 域名" prop="tls_domain">
                <el-input
                  v-model="form.tls_domain"
                  placeholder="如 jp.example.com"
                />
                <div class="field-tip">
                  用于节点 TLS 标识、健康检查和证书自动匹配。
                </div>
              </el-form-item>

              <el-form-item class="field-span-2" label="系统证书">
                <el-select
                  v-model="form.certificate_id"
                  filterable
                  clearable
                  class="full-width-control"
                  :loading="certificatesLoading"
                  placeholder="自动匹配或手动选择证书"
                  @change="handleCertificateChange"
                >
                  <el-option
                    v-for="cert in certificates"
                    :key="cert.id"
                    :label="getCertificateOptionLabel(cert)"
                    :value="cert.id"
                  />
                </el-select>
                <div class="field-tip">
                  选择证书后会自动回填 TLS 域名，你仍可继续手动修改。
                </div>
                <div v-if="selectedCertificate" class="certificate-tip">
                  当前证书：{{ selectedCertificate.domain }}
                  <span
                    v-if="
                      selectedCertificate.expireDate &&
                      selectedCertificate.expireDate !== '-'
                    "
                  >
                    ，到期 {{ selectedCertificate.expireDate }}
                  </span>
                </div>
              </el-form-item>
            </div>
          </el-card>

          <el-card v-if="!isEdit" class="section-card" shadow="never">
            <template #header>
              <div class="section-head section-head--split">
                <div>
                  <h2 class="section-title">自动部署</h2>
                  <p class="section-subtitle">
                    通过 SSH 自动安装 Agent 与 Xray，适合新节点首次接入。
                  </p>
                </div>
                <el-switch
                  v-model="enableAutoInstall"
                  active-text="开启"
                  inactive-text="关闭"
                />
              </div>
            </template>

            <div class="section-note">
              {{
                enableAutoInstall
                  ? "已开启自动部署，请继续补充 SSH 信息。"
                  : "关闭时仅保存节点配置，不执行远程安装。"
              }}
            </div>

            <div v-if="enableAutoInstall" class="form-grid">
              <el-form-item label="服务器 IP">
                <el-input
                  v-model="form.ssh_host"
                  :placeholder="
                    normalizeText(form.address)
                      ? `默认使用 ${normalizeText(form.address)}`
                      : '默认使用上方节点地址'
                  "
                  @input="handleSSHHostInput"
                />
              </el-form-item>

              <el-form-item label="SSH 端口">
                <el-input-number
                  v-model="form.ssh_port"
                  class="full-width-input"
                  :min="1"
                  :max="65535"
                />
              </el-form-item>

              <el-form-item label="SSH 用户名">
                <el-input
                  v-model="form.ssh_username"
                  placeholder="通常为 root"
                />
              </el-form-item>

              <el-form-item label="认证方式">
                <el-radio-group v-model="form.ssh_auth_type">
                  <el-radio value="password"> 密码 </el-radio>
                  <el-radio value="key"> 私钥 </el-radio>
                </el-radio-group>
              </el-form-item>

              <el-form-item
                v-if="form.ssh_auth_type === 'password'"
                class="field-span-2"
                label="SSH 密码"
              >
                <el-input
                  v-model="form.ssh_password"
                  type="password"
                  placeholder="SSH 登录密码"
                  show-password
                />
              </el-form-item>

              <el-form-item
                v-if="form.ssh_auth_type === 'key'"
                class="field-span-2"
                label="SSH 私钥"
              >
                <el-input
                  v-model="form.ssh_private_key"
                  type="textarea"
                  :rows="6"
                  placeholder="粘贴 SSH 私钥内容"
                />
              </el-form-item>

              <div class="field-span-2 section-actions">
                <el-button
                  plain
                  :loading="testingConnection"
                  @click="testSSHConnection"
                >
                  测试 SSH 连接
                </el-button>
              </div>
            </div>
          </el-card>
        </div>

        <div class="form-side">
          <el-card class="summary-card" shadow="never">
            <div class="summary-card__head">
              <span class="summary-card__eyebrow">智能概览</span>
              <strong class="summary-card__title">{{
                suggestedNameDisplay
              }}</strong>
              <p class="summary-card__description">
                {{ summaryDescription }}
              </p>
            </div>

            <div class="summary-list">
              <div
                v-for="item in summaryItems"
                :key="item.label"
                class="summary-list__item"
              >
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
              </div>
            </div>

            <div class="summary-highlight">
              <span>提交前检查</span>
              <p>{{ summaryChecklist }}</p>
            </div>
          </el-card>

          <el-card class="summary-card action-card" shadow="never">
            <div class="action-card__title">保存节点</div>
            <p class="action-card__text">
              {{
                isEdit
                  ? "更新后会保留现有节点身份、流量与监控记录。"
                  : "创建成功后可继续进行部署、分组和订阅配置。"
              }}
            </p>
            <el-button
              type="primary"
              size="large"
              class="action-card__primary"
              :loading="submitting"
              @click="submitForm"
            >
              {{ pageActionText }}
            </el-button>
            <el-button
              size="large"
              class="action-card__secondary"
              @click="goBack"
            >
              取消
            </el-button>
          </el-card>
        </div>
      </div>
    </el-form>

    <!-- 创建成功后显示 Token -->
    <el-dialog
      v-model="tokenDialogVisible"
      title="节点创建成功"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-alert type="success" :closable="false" show-icon>
        <template #title> 节点已创建成功！ </template>
        请保存以下 Token，用于 Node Agent 连接认证。此 Token 只显示一次。
      </el-alert>
      <div class="token-display">
        <div class="token-label">认证 Token</div>
        <div class="token-value">
          <code>{{ createdToken }}</code>
          <el-button link @click="copyToken">
            <el-icon><CopyDocument /></el-icon>
            复制
          </el-button>
        </div>
      </div>
      <div class="agent-config">
        <div class="config-label">Agent 配置示例</div>
        <pre class="config-code">{{ agentConfigExample }}</pre>
      </div>
      <template #footer>
        <el-button type="primary" @click="finishCreate">
          我已保存，完成
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import {
  ref,
  reactive,
  computed,
  onMounted,
  onBeforeUnmount,
  nextTick,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { ArrowLeft, CopyDocument } from "@element-plus/icons-vue";
import { useNodeStore } from "@/stores/node";
import { certificatesApi, nodeGroupsApi, nodesApi } from "@/api";
import {
  formatNodeTrafficRemaining,
  formatNodeTrafficResetAt,
  formatNodeTrafficUsageSummary,
  getNodeTrafficStateKey,
  getNodeTrafficStateText,
  getNodeTrafficUsagePercent,
  hasNodeTrafficLimit,
} from "@/composables/useNodePresentation";
import { copyText } from "@/utils/clipboard";
import { debounce } from "@/utils/debounce";
import { extractErrorMessage } from "@/utils/entitlement";

const route = useRoute();
const router = useRouter();
const nodeStore = useNodeStore();

const isEdit = computed(() => !!route.params.id);
const pageTitle = computed(() => (isEdit.value ? "编辑节点" : "添加节点"));
const pageDescription = computed(() =>
  isEdit.value
    ? "调整节点接入、TLS、分组与部署参数，保持线上节点配置整洁一致。"
    : "把节点基础接入、智能命名、TLS 与部署配置集中在一个工作区里完成。",
);
const loading = ref(false);
const submitting = ref(false);
const formRef = ref(null);
const tagInputRef = ref(null);
const showTagInput = ref(false);
const newTag = ref("");
const addressType = ref("ip");
const groups = ref([]);
const certificates = ref([]);
const certificatesLoading = ref(false);
const tokenDialogVisible = ref(false);
const createdToken = ref("");
const suggestionLoading = ref(false);
const suggestionEnabled = ref(false);
const nameSuggestionMessage = ref(
  "填写地址后会自动生成节点名称，你也可以手动覆盖。",
);
const isNameManuallyEdited = ref(false);
const isApplyingSuggestedName = ref(false);
const lastSuggestedName = ref("");
let latestSuggestionRequestId = 0;

// SSH 自动安装相关
const enableAutoInstall = ref(false);
const testingConnection = ref(false);
const isSSHHostManuallyEdited = ref(false);
const isApplyingSSHHostDefault = ref(false);
const currentTrafficTotal = ref(0);
const currentTrafficResetAt = ref("");

const trafficLimitUnitOptions = [
  { label: "GiB", value: "GiB" },
  { label: "TiB", value: "TiB" },
];

const TRAFFIC_LIMIT_UNIT_MULTIPLIERS = Object.freeze({
  GiB: 1024 ** 3,
  TiB: 1024 ** 4,
});

const DEFAULT_TLS_DOMAIN = "www.shcrystal.top";

const form = reactive({
  name: "",
  address: "",
  port: 18443,
  region: "",
  weight: 1,
  max_users: 0,
  tags: [],
  ip_whitelist_str: "",
  group_ids: [],
  traffic_limit_value: 0,
  traffic_limit_unit: "GiB",
  alert_traffic_threshold: 80,
  tls_enabled: true,
  tls_domain: DEFAULT_TLS_DOMAIN,
  certificate_id: null,
  // SSH 连接信息
  ssh_host: "",
  ssh_port: 22,
  ssh_username: "root",
  ssh_auth_type: "password",
  ssh_password: "",
  ssh_private_key: "",
});

const pageActionText = computed(() =>
  isEdit.value
    ? "保存修改"
    : enableAutoInstall.value
      ? "创建并安装"
      : "创建节点",
);

const rules = {
  name: [
    { required: true, message: "请输入节点名称", trigger: "blur" },
    {
      min: 2,
      max: 128,
      message: "名称长度在 2 到 128 个字符",
      trigger: "blur",
    },
  ],
  address: [
    { required: true, message: "请输入节点地址", trigger: "blur" },
    { validator: validateAddress, trigger: "blur" },
  ],
  port: [{ required: true, message: "请输入端口", trigger: "blur" }],
  tls_domain: [{ validator: validateTLSDomain, trigger: "blur" }],
};

const normalizeText = (value) =>
  typeof value === "string" ? value.trim() : "";

const getDefaultSSHHost = () => normalizeText(form.address);

const getEffectiveSSHHost = () =>
  normalizeText(form.ssh_host) || getDefaultSSHHost();

const syncSSHHostWithNodeAddress = () => {
  if (isEdit.value || !enableAutoInstall.value) return;

  const defaultHost = getDefaultSSHHost();
  const currentHost = normalizeText(form.ssh_host);
  if (currentHost && currentHost === defaultHost) {
    isSSHHostManuallyEdited.value = false;
  }
  if (isSSHHostManuallyEdited.value) return;

  if (form.ssh_host !== defaultHost) {
    isApplyingSSHHostDefault.value = true;
    form.ssh_host = defaultHost;
    isApplyingSSHHostDefault.value = false;
  }
};

const handleSSHHostInput = () => {
  if (isApplyingSSHHostDefault.value) return;

  const currentHost = normalizeText(form.ssh_host);
  if (!currentHost) {
    isSSHHostManuallyEdited.value = false;
    syncSSHHostWithNodeAddress();
    return;
  }

  isSSHHostManuallyEdited.value = currentHost !== getDefaultSSHHost();
};

const namingModeLabel = computed(() => {
  if (suggestionLoading.value) return "识别中";
  if (isNameManuallyEdited.value) return "手动优先";
  if (lastSuggestedName.value) return "自动命名";
  return "等待输入";
});

const deploymentModeLabel = computed(() => {
  if (isEdit.value) return "配置维护";
  return enableAutoInstall.value ? "SSH 自动安装" : "手动接入";
});

const suggestedNameDisplay = computed(
  () => normalizeText(form.name) || lastSuggestedName.value || "等待地址识别",
);

const getTrafficLimitBytes = () => {
  const value = Number(form.traffic_limit_value);
  if (!Number.isFinite(value) || value <= 0) return 0;

  const multiplier =
    TRAFFIC_LIMIT_UNIT_MULTIPLIERS[form.traffic_limit_unit] ||
    TRAFFIC_LIMIT_UNIT_MULTIPLIERS.GiB;
  return Math.round(value * multiplier);
};

const applyTrafficLimitForm = (bytes) => {
  const normalized = Number(bytes) || 0;
  if (normalized <= 0) {
    form.traffic_limit_value = 0;
    form.traffic_limit_unit = "GiB";
    return;
  }

  if (normalized % TRAFFIC_LIMIT_UNIT_MULTIPLIERS.TiB === 0) {
    form.traffic_limit_unit = "TiB";
    form.traffic_limit_value = Number(
      (normalized / TRAFFIC_LIMIT_UNIT_MULTIPLIERS.TiB).toFixed(2),
    );
    return;
  }

  form.traffic_limit_unit = "GiB";
  const gibValue = normalized / TRAFFIC_LIMIT_UNIT_MULTIPLIERS.GiB;
  form.traffic_limit_value = Number(
    gibValue >= 100 ? gibValue.toFixed(0) : gibValue.toFixed(2),
  );
};

const previewTrafficNode = computed(() => ({
  traffic_total: currentTrafficTotal.value,
  traffic_limit: getTrafficLimitBytes(),
  alert_traffic_threshold: form.alert_traffic_threshold,
}));

const hasPreviewTrafficLimit = computed(() =>
  hasNodeTrafficLimit(previewTrafficNode.value),
);
const trafficQuotaSummary = computed(() =>
  formatNodeTrafficUsageSummary(previewTrafficNode.value),
);
const trafficQuotaRemaining = computed(() =>
  formatNodeTrafficRemaining(previewTrafficNode.value),
);
const trafficQuotaPercent = computed(() =>
  getNodeTrafficUsagePercent(previewTrafficNode.value),
);
const trafficQuotaStateText = computed(() =>
  getNodeTrafficStateText(previewTrafficNode.value),
);
const currentTrafficResetLabel = computed(() =>
  formatNodeTrafficResetAt(currentTrafficResetAt.value),
);
const trafficQuotaProgressStatus = computed(() => {
  const state = getNodeTrafficStateKey(previewTrafficNode.value);
  if (state === "hard_capped") return "exception";
  if (state === "soft_capped") return "warning";
  return "success";
});

const summaryDescription = computed(() =>
  normalizeText(form.address)
    ? nameSuggestionMessage.value
    : "填写节点地址后，这里会展示智能命名和关键配置摘要。",
);

const summaryItems = computed(() => [
  {
    label: "地址类型",
    value: addressType.value === "ip" ? "IP 地址" : "域名",
  },
  {
    label: "节点地址",
    value: normalizeText(form.address) || "未填写",
  },
  {
    label: "Agent 端口",
    value: String(form.port || "-"),
  },
  {
    label: "月流量",
    value: trafficQuotaSummary.value,
  },
  {
    label: "流量状态",
    value: trafficQuotaStateText.value,
  },
  {
    label: "TLS 状态",
    value: form.tls_enabled
      ? normalizeText(form.tls_domain)
        ? `已开启 · ${normalizeText(form.tls_domain)}`
        : "已开启"
      : "未开启",
  },
  {
    label: "所属分组",
    value: form.group_ids.length ? `${form.group_ids.length} 个` : "未选择",
  },
  {
    label: "自动部署",
    value: isEdit.value
      ? "编辑模式"
      : enableAutoInstall.value
        ? "已开启"
        : "未开启",
  },
]);

const summaryChecklist = computed(() => {
  if (!normalizeText(form.address)) {
    return "先填写节点地址，系统会自动生成推荐名称并识别地区。";
  }
  if (!normalizeText(form.name)) {
    return "请确认节点名称，或点击“自动生成”回填推荐名称。";
  }
  if (form.tls_enabled && !normalizeText(form.tls_domain)) {
    return "已开启 TLS，建议补充 TLS 域名并确认系统证书。";
  }
  if (hasPreviewTrafficLimit.value && !isEdit.value) {
    return "已启用节点月流量保护，创建后会从当前时间开始首个计费周期。";
  }
  if (
    !isEdit.value &&
    enableAutoInstall.value &&
    !getEffectiveSSHHost()
  ) {
    return "自动部署已开启，请继续填写服务器 SSH 信息。";
  }
  return "基础信息已完整，可以继续保存或直接创建节点。";
});

const isManualNamingState = computed(
  () =>
    isNameManuallyEdited.value &&
    normalizeText(form.name) !== lastSuggestedName.value,
);

const suggestionTone = computed(() => {
  if (suggestionLoading.value) return "is-neutral";
  if (nameSuggestionMessage.value.includes("失败")) return "is-danger";
  if (isManualNamingState.value) {
    return "is-warning";
  }
  if (normalizeText(form.address) && normalizeText(form.name)) {
    return "is-success";
  }
  return "is-neutral";
});

const suggestionHeadline = computed(() => {
  if (suggestionLoading.value) return "智能识别中";
  if (suggestionTone.value === "is-danger") return "命名识别异常";
  if (suggestionTone.value === "is-warning") return "已切换手动命名";
  if (normalizeText(form.address) && normalizeText(form.name))
    return "命名建议";
  return "等待地址输入";
});

const clearFieldValidation = (fields) => {
  nextTick(() => {
    formRef.value?.clearValidate(fields);
  });
};

const isValidIPv4Address = (value) => /^(\d{1,3}\.){3}\d{1,3}$/.test(value);

const isValidIPv6Address = (value) =>
  /^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$|^::$|^([0-9a-fA-F]{1,4}:)*::([0-9a-fA-F]{1,4}:)*[0-9a-fA-F]{1,4}$/.test(
    value,
  );

const isValidDomain = (value) =>
  /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/.test(
    value,
  );

function validateAddress(rule, value, callback) {
  if (!value) {
    callback(new Error("请输入节点地址"));
    return;
  }

  if (addressType.value === "ip") {
    if (!isValidIPv4Address(value) && !isValidIPv6Address(value)) {
      callback(new Error("请输入有效的 IP 地址"));
      return;
    }
  } else {
    if (!isValidDomain(value)) {
      callback(new Error("请输入有效的域名"));
      return;
    }
  }
  callback();
}

const normalizeCertificateDomain = (domain) => {
  const normalized = normalizeText(domain).toLowerCase();
  if (!normalized) return "";
  if (normalized === "*.shcrystal.top") return DEFAULT_TLS_DOMAIN;
  return normalized.replace(/^\*\./, "");
};

const formatCertificateDate = (value) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toISOString().slice(0, 10);
};

const normalizeCertificatesResponse = (response) => {
  if (Array.isArray(response)) return response;
  if (Array.isArray(response?.certificates)) return response.certificates;
  if (Array.isArray(response?.data?.certificates))
    return response.data.certificates;
  if (Array.isArray(response?.data)) return response.data;
  return [];
};

const selectedCertificate = computed(
  () =>
    certificates.value.find(
      (cert) => Number(cert.id) === Number(form.certificate_id),
    ) || null,
);

function validateTLSDomain(rule, value, callback) {
  const domain = normalizeText(value).toLowerCase();
  const domainRegex =
    /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/;

  if (form.tls_enabled && !domain) {
    callback(new Error("启用 TLS 时请输入 TLS 域名"));
    return;
  }

  if (domain && !domainRegex.test(domain)) {
    callback(new Error("请输入有效的 TLS 域名"));
    return;
  }

  callback();
}

const agentConfigExample = computed(() => {
  return `# agent.yaml
panel:
  url: "${window.location.origin}"
  
node:
  name: "${form.name}"
  token: "${createdToken.value}"
  
xray:
  config_path: "/usr/local/etc/xray/config.json"`;
});

const buildSuggestionDetail = (result) => {
  const parts = [];
  if (normalizeText(result?.location_label)) {
    parts.push(result.location_label);
  } else if (normalizeText(result?.suggested_region)) {
    parts.push(result.suggested_region);
  }
  if (normalizeText(result?.resolved_ip)) {
    parts.push(result.resolved_ip);
  }
  return parts.join(" / ");
};

const canSuggestNodeName = () => {
  const address = normalizeText(form.address);
  if (!address) return false;
  return addressType.value === "ip"
    ? isValidIPv4Address(address) || isValidIPv6Address(address)
    : isValidDomain(address);
};

const setNameSuggestionMessage = (message) => {
  nameSuggestionMessage.value =
    message || "填写地址后会自动生成节点名称，你也可以手动覆盖。";
};

const applySuggestedName = (result, force = false) => {
  const previousSuggestedName = lastSuggestedName.value;
  lastSuggestedName.value = normalizeText(result?.suggested_name);

  if (!normalizeText(form.region) && normalizeText(result?.suggested_region)) {
    form.region = result.suggested_region;
  }

  const shouldOverwriteName =
    force ||
    !normalizeText(form.name) ||
    !isNameManuallyEdited.value ||
    normalizeText(form.name) === previousSuggestedName;

  const detail = buildSuggestionDetail(result);
  if (shouldOverwriteName && lastSuggestedName.value) {
    isApplyingSuggestedName.value = true;
    form.name = lastSuggestedName.value;
    isApplyingSuggestedName.value = false;
    isNameManuallyEdited.value = false;
    clearFieldValidation(["name", "region"]);
    setNameSuggestionMessage(
      detail
        ? `已识别 ${detail}，节点名称已自动生成。`
        : `节点名称已自动生成：${lastSuggestedName.value}`,
    );
    return;
  }

  setNameSuggestionMessage(
    detail
      ? `已识别 ${detail}，已保留当前手动名称。点击“自动生成”可重新回填。`
      : "已保留当前手动名称。点击“自动生成”可重新回填。",
  );
};

const generateSuggestedName = async (force = false) => {
  const address = normalizeText(form.address);
  if (!address) {
    suggestionLoading.value = false;
    if (!normalizeText(form.name)) {
      lastSuggestedName.value = "";
    }
    setNameSuggestionMessage();
    return;
  }

  if (!canSuggestNodeName()) {
    suggestionLoading.value = false;
    setNameSuggestionMessage("地址格式可用后会自动生成节点名称。");
    return;
  }

  const requestId = ++latestSuggestionRequestId;
  suggestionLoading.value = true;
  if (force) {
    setNameSuggestionMessage("正在根据地址解析节点名称...");
  }

  try {
    const result = await nodesApi.suggestName({
      address,
      region: normalizeText(form.region) || undefined,
    });
    if (requestId !== latestSuggestionRequestId) return;
    applySuggestedName(result, force);
  } catch (e) {
    if (requestId !== latestSuggestionRequestId) return;
    setNameSuggestionMessage(
      force
        ? "自动生成失败，请检查地址后重试。"
        : "地址已更新，但名称识别失败，可稍后点击“自动生成”重试。",
    );
    if (force) {
      ElMessage.error(e.message || "自动生成失败");
    }
  } finally {
    if (requestId === latestSuggestionRequestId) {
      suggestionLoading.value = false;
    }
  }
};

const debouncedSuggestName = debounce(() => {
  void generateSuggestedName(false);
}, 500);

const handleAddressBlur = () => {
  if (!suggestionEnabled.value) return;
  debouncedSuggestName.cancel();
  void generateSuggestedName(false);
};

const handleNameInput = () => {
  if (!suggestionEnabled.value || isApplyingSuggestedName.value) {
    return;
  }

  if (normalizeText(form.name) === lastSuggestedName.value) {
    isNameManuallyEdited.value = false;
    clearFieldValidation(["name"]);
    return;
  }

  isNameManuallyEdited.value = true;
  if (normalizeText(form.name)) {
    clearFieldValidation(["name"]);
  }
  setNameSuggestionMessage("已切换为手动命名，点击“自动生成”可重新回填。");
};

const fetchGroups = async () => {
  try {
    const res = await nodeGroupsApi.list();
    groups.value = res?.groups || res || [];
  } catch (e) {
    console.error("获取分组失败:", e);
  }
};

const fetchCertificates = async () => {
  certificatesLoading.value = true;
  try {
    const response = await certificatesApi.list();
    certificates.value = normalizeCertificatesResponse(response).map(
      (cert) => ({
        ...cert,
        expireDate: formatCertificateDate(cert.expires_at || cert.expiresAt),
      }),
    );
  } catch (e) {
    console.error("获取证书失败:", e);
    certificates.value = [];
  } finally {
    certificatesLoading.value = false;
  }
};

const getCertificateOptionLabel = (cert) => {
  if (!cert) return "";
  return cert.expireDate && cert.expireDate !== "-"
    ? `${cert.domain}（到期 ${cert.expireDate}）`
    : cert.domain;
};

const handleCertificateChange = (certificateId) => {
  if (!certificateId) return;
  const certificate = certificates.value.find(
    (cert) => Number(cert.id) === Number(certificateId),
  );
  if (!certificate) return;

  const suggestedDomain = normalizeCertificateDomain(certificate.domain);
  form.tls_enabled = true;
  if (suggestedDomain) {
    form.tls_domain = suggestedDomain;
  }
};

const fetchNode = async () => {
  if (!isEdit.value) return;

  loading.value = true;
  try {
    const node = await nodeStore.fetchNode(route.params.id);

    // 填充表单
    form.name = node.name;
    form.address = node.address;
    form.port = node.port;
    form.region = node.region || "";
    form.weight = node.weight || 1;
    form.max_users = node.max_users || 0;
    form.tags = parseTags(node.tags);
    form.ip_whitelist_str = node.ip_whitelist
      ? Array.isArray(node.ip_whitelist)
        ? node.ip_whitelist.join("\n")
        : ""
      : "";
    form.group_ids = Array.isArray(node.group_ids)
      ? node.group_ids
      : node.group_id
        ? [node.group_id]
        : [];
    applyTrafficLimitForm(node.traffic_limit);
    form.alert_traffic_threshold = Number(node.alert_traffic_threshold) || 80;
    currentTrafficTotal.value = Number(node.traffic_total) || 0;
    currentTrafficResetAt.value = node.traffic_reset_at || "";
    form.tls_enabled = Boolean(node.tls_enabled);
    form.tls_domain = node.tls_domain || "";
    form.certificate_id = node.certificate_id ?? null;

    // 判断地址类型
    const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$|^([0-9a-fA-F]{1,4}:)/;
    addressType.value = ipRegex.test(node.address) ? "ip" : "domain";
    isNameManuallyEdited.value = Boolean(normalizeText(node.name));
    lastSuggestedName.value = "";
    setNameSuggestionMessage(
      "当前名称视为手动命名，修改地址或地区后不会自动覆盖，可点击“自动生成”重新回填。",
    );
  } catch (e) {
    ElMessage.error(e.message || "获取节点详情失败");
  } finally {
    loading.value = false;
  }
};

const parseTags = (tags) => {
  if (Array.isArray(tags)) return tags;
  if (typeof tags === "string") {
    try {
      return JSON.parse(tags);
    } catch (e) {
      console.warn("Failed to parse tags JSON:", e);
      return [];
    }
  }
  return [];
};

const showTagInputField = () => {
  showTagInput.value = true;
  nextTick(() => {
    tagInputRef.value?.focus();
  });
};

const addTag = () => {
  if (newTag.value.trim() && !form.tags.includes(newTag.value.trim())) {
    form.tags.push(newTag.value.trim());
  }
  newTag.value = "";
  showTagInput.value = false;
};

const removeTag = (index) => {
  form.tags.splice(index, 1);
};

const submitForm = async () => {
  const isFormValid = await formRef.value
    .validate()
    .then(() => true)
    .catch(() => false);

  if (!isFormValid) {
    return;
  }

  if (form.tls_enabled && !normalizeText(form.tls_domain)) {
    ElMessage.error("启用 TLS 时请输入 TLS 域名");
    return;
  }

  submitting.value = true;
  try {
    const ipWhitelist = form.ip_whitelist_str
      .split("\n")
      .map((s) => s.trim())
      .filter(Boolean);

    const data = {
      name: form.name,
      address: form.address,
      port: form.port,
      region: form.region,
      weight: form.weight,
      max_users: form.max_users,
      tags: form.tags,
      ip_whitelist: ipWhitelist,
      group_id: form.group_ids[0] || null,
      group_ids: form.group_ids,
      traffic_limit: getTrafficLimitBytes(),
      alert_traffic_threshold: form.alert_traffic_threshold,
      tls_enabled: form.tls_enabled,
      tls_domain: normalizeText(form.tls_domain).toLowerCase(),
      certificate_id: form.certificate_id || null,
    };

    // 如果开启了自动安装，添加 SSH 信息
    if (enableAutoInstall.value) {
      const sshHost = getEffectiveSSHHost();
      if (!sshHost) {
        ElMessage.error("请先填写节点地址或服务器 IP");
        return;
      }
      if (form.ssh_auth_type === "password" && !form.ssh_password) {
        ElMessage.error("请输入 SSH 密码");
        return;
      }
      if (form.ssh_auth_type === "key" && !form.ssh_private_key) {
        ElMessage.error("请输入 SSH 私钥");
        return;
      }

      data.ssh = {
        host: sshHost,
        port: form.ssh_port,
        username: form.ssh_username,
        password: form.ssh_auth_type === "password" ? form.ssh_password : "",
        private_key: form.ssh_auth_type === "key" ? form.ssh_private_key : "",
      };
    }

    if (isEdit.value) {
      await nodeStore.updateNode(route.params.id, data);
      ElMessage.success("更新成功");
      router.push("/admin/nodes");
    } else {
      const res = await nodeStore.createNode(data);

      // 如果已启动后台自动安装
      if (res.installing) {
        ElMessage.success(res.message || "节点创建成功，后台自动安装已开始");
        router.push("/admin/nodes");
      } else if (res.token) {
        // 没有自动安装，显示 Token
        createdToken.value = res.token;
        tokenDialogVisible.value = true;
      } else {
        ElMessage.success("创建成功");
        router.push("/admin/nodes");
      }
    }
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || "操作失败");
  } finally {
    submitting.value = false;
  }
};

const copyToken = async () => {
  try {
    await copyText(createdToken.value);
    ElMessage.success("已复制到剪贴板");
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || "复制失败");
  }
};

// 测试 SSH 连接
const testSSHConnection = async () => {
  const sshHost = getEffectiveSSHHost();
  if (!sshHost) {
    ElMessage.error("请先填写节点地址或服务器 IP");
    return;
  }
  if (form.ssh_auth_type === "password" && !form.ssh_password) {
    ElMessage.error("请输入 SSH 密码");
    return;
  }
  if (form.ssh_auth_type === "key" && !form.ssh_private_key) {
    ElMessage.error("请输入 SSH 私钥");
    return;
  }

  testingConnection.value = true;
  try {
    const res = await nodesApi.testConnection({
      host: sshHost,
      port: form.ssh_port,
      username: form.ssh_username,
      password: form.ssh_auth_type === "password" ? form.ssh_password : "",
      private_key: form.ssh_auth_type === "key" ? form.ssh_private_key : "",
    });

    if (res.success) {
      ElMessage.success("SSH 连接测试成功");
    } else {
      ElMessage.error(res.message || "SSH 连接失败");
    }
  } catch (e) {
    ElMessage.error(e.message || "SSH 连接测试失败");
  } finally {
    testingConnection.value = false;
  }
};

const finishCreate = () => {
  tokenDialogVisible.value = false;
  router.push("/admin/nodes");
};

const goBack = () => {
  router.push("/admin/nodes");
};

watch(
  () => [
    normalizeText(form.address),
    normalizeText(form.region),
    addressType.value,
  ],
  ([address]) => {
    if (!suggestionEnabled.value) return;
    if (!address) {
      lastSuggestedName.value = "";
      if (!normalizeText(form.name) || !isNameManuallyEdited.value) {
        setNameSuggestionMessage();
      }
      return;
    }
    debouncedSuggestName();
  },
);

watch(
  () => [enableAutoInstall.value, normalizeText(form.address)],
  () => {
    syncSSHHostWithNodeAddress();
  },
);

onMounted(async () => {
  await Promise.all([fetchGroups(), fetchCertificates()]);
  await fetchNode();
  suggestionEnabled.value = true;

  if (!isEdit.value && normalizeText(form.address)) {
    void generateSuggestedName(false);
  }
});

onBeforeUnmount(() => {
  debouncedSuggestName.cancel();
  latestSuggestionRequestId += 1;
});
</script>

<style scoped>
.node-form-page {
  --node-ink: #182033;
  --node-muted: #68748c;
  --node-line: rgba(129, 145, 178, 0.18);
  --node-surface: rgba(255, 255, 255, 0.92);
  --node-accent: #2a66df;
  --node-accent-soft: rgba(42, 102, 223, 0.1);
  min-height: 100%;
  padding: 24px;
  background:
    radial-gradient(
      circle at top left,
      rgba(42, 102, 223, 0.09),
      transparent 24%
    ),
    radial-gradient(
      circle at top right,
      rgba(245, 158, 11, 0.08),
      transparent 20%
    ),
    linear-gradient(180deg, #f4f7fb 0%, #eef3f9 100%);
}

.hero-panel {
  max-width: 1360px;
  margin: 0 auto 24px;
  padding: 28px 32px;
  border-radius: 30px;
  border: 1px solid var(--node-line);
  background: linear-gradient(
    135deg,
    rgba(255, 255, 255, 0.98),
    rgba(245, 248, 255, 0.94)
  );
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.08);
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  flex-wrap: wrap;
}

.hero-panel__nav {
  display: flex;
  align-items: flex-start;
  gap: 24px;
  flex: 1 1 720px;
}

.back-link {
  padding: 10px 0;
  color: var(--node-muted);
}

.hero-panel__copy {
  display: grid;
  gap: 10px;
}

.hero-panel__eyebrow {
  font-size: 11px;
  letter-spacing: 0.24em;
  text-transform: uppercase;
  color: var(--node-muted);
}

.page-title {
  margin: 0;
  font-size: clamp(36px, 5vw, 56px);
  line-height: 1;
  color: var(--node-ink);
  font-weight: 700;
  font-family: "Bahnschrift", "Avenir Next", "PingFang SC", sans-serif;
}

.page-subtitle {
  margin: 0;
  max-width: 760px;
  color: var(--node-muted);
  font-size: 15px;
  line-height: 1.7;
}

.hero-panel__stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  min-width: min(100%, 360px);
  flex: 1 1 340px;
}

.hero-stat {
  padding: 16px 18px;
  border-radius: 22px;
  border: 1px solid rgba(129, 145, 178, 0.16);
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(10px);
  display: grid;
  gap: 8px;
}

.hero-stat__label {
  font-size: 12px;
  color: var(--node-muted);
}

.hero-stat__value {
  font-size: 18px;
  line-height: 1.2;
  color: var(--node-ink);
}

.node-form {
  max-width: 1360px;
  margin: 0 auto;
}

.form-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 24px;
  align-items: start;
}

.form-main {
  display: grid;
  gap: 20px;
}

.form-side {
  position: sticky;
  top: 24px;
  display: grid;
  gap: 20px;
}

.section-card,
.summary-card {
  border-radius: 28px;
  border: 1px solid var(--node-line);
  background: var(--node-surface);
  box-shadow: 0 16px 42px rgba(15, 23, 42, 0.06);
}

.section-card :deep(.el-card__header),
.summary-card :deep(.el-card__header) {
  border-bottom: none;
}

.section-card :deep(.el-card__header) {
  padding: 24px 26px 0;
}

.section-card :deep(.el-card__body) {
  padding: 24px 26px 28px;
}

.summary-card :deep(.el-card__body) {
  padding: 24px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.section-head--split {
  align-items: center;
}

.section-title {
  margin: 0;
  font-size: 22px;
  color: var(--node-ink);
  font-weight: 700;
}

.section-subtitle {
  margin: 8px 0 0;
  color: var(--node-muted);
  font-size: 13px;
  line-height: 1.6;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px 18px;
}

.field-span-2 {
  grid-column: 1 / -1;
}

.node-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.node-form :deep(.el-form-item__label) {
  padding-bottom: 8px;
  color: var(--node-ink);
  font-weight: 700;
  line-height: 1.2;
}

.node-form :deep(.el-form-item__content) {
  display: block;
}

.node-form :deep(.el-form-item__error) {
  position: static;
  margin-top: 8px;
  line-height: 1.5;
}

.node-form :deep(.el-input__wrapper),
.node-form :deep(.el-select__wrapper),
.node-form :deep(.el-textarea__inner),
.node-form :deep(.el-input-number),
.node-form :deep(.el-input-number .el-input__wrapper) {
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 0 0 1px rgba(129, 145, 178, 0.18) inset;
}

.node-form :deep(.el-input__wrapper.is-focus),
.node-form :deep(.el-select__wrapper.is-focused),
.node-form :deep(.el-textarea__inner:focus),
.node-form :deep(.el-input-number:focus-within) {
  box-shadow:
    0 0 0 1px rgba(42, 102, 223, 0.38),
    0 10px 24px rgba(42, 102, 223, 0.12);
}

.name-field {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.name-field :deep(.el-input) {
  flex: 1;
}

.name-field__action {
  min-width: 132px;
  height: 50px;
  border-radius: 18px;
}

.address-field {
  display: grid;
  gap: 12px;
}

.address-field__controls {
  display: grid;
  grid-template-columns: 140px minmax(0, 1fr);
  gap: 12px;
  align-items: stretch;
}

.address-type-switch {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  padding: 6px;
  border-radius: 22px;
  background: rgba(244, 247, 251, 0.92);
  box-shadow: 0 0 0 1px rgba(129, 145, 178, 0.16) inset;
}

.address-type-switch :deep(.el-radio-button) {
  width: 100%;
}

.address-type-switch :deep(.el-radio-button__original-radio) {
  pointer-events: none;
}

.address-type-switch :deep(.el-radio-button__inner) {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50px;
  border: none;
  border-radius: 16px;
  background: transparent;
  box-shadow: none;
  color: var(--node-muted);
  font-weight: 700;
  transition:
    background-color 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease;
}

.address-type-switch
  :deep(.el-radio-button:first-child .el-radio-button__inner),
.address-type-switch
  :deep(.el-radio-button:last-child .el-radio-button__inner) {
  border-radius: 16px;
}

.address-type-switch
  :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  background: linear-gradient(135deg, #2a66df, #1f8ce6);
  color: #fff;
  box-shadow: 0 10px 20px rgba(42, 102, 223, 0.22);
}

.address-field__input :deep(.el-input__wrapper) {
  min-height: 62px;
  padding-inline: 18px;
}

.address-field__input :deep(.el-input__inner) {
  font-size: 17px;
}

.address-field__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.address-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 30px;
  padding: 0 12px;
  border-radius: 999px;
  background: rgba(129, 145, 178, 0.1);
  color: var(--node-muted);
  font-size: 12px;
  font-weight: 600;
}

.address-badge.is-active {
  background: rgba(42, 102, 223, 0.12);
  color: var(--node-accent);
}

.suggestion-banner {
  margin-top: 12px;
  padding: 14px 16px;
  border-radius: 18px;
  border: 1px solid transparent;
  display: grid;
  gap: 6px;
}

.suggestion-banner__label {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.suggestion-banner__text {
  font-size: 13px;
  line-height: 1.6;
}

.suggestion-banner.is-success {
  background: rgba(42, 102, 223, 0.1);
  border-color: rgba(42, 102, 223, 0.16);
  color: #315ea6;
}

.suggestion-banner.is-warning {
  background: rgba(245, 158, 11, 0.14);
  border-color: rgba(245, 158, 11, 0.18);
  color: #925d05;
}

.suggestion-banner.is-danger {
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.16);
  color: #b42318;
}

.suggestion-banner.is-neutral {
  background: rgba(100, 116, 139, 0.08);
  border-color: rgba(100, 116, 139, 0.14);
  color: #5b6477;
}

.field-tip,
.field-tip-inline {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--node-muted);
}

.full-width-input,
.full-width-control {
  width: 100%;
}

.slider-field {
  display: flex;
  align-items: center;
  gap: 16px;
}

.slider-field__control {
  flex: 1;
}

.slider-pill {
  flex: none;
  min-width: 92px;
  padding: 11px 14px;
  border-radius: 999px;
  background: var(--node-accent-soft);
  color: var(--node-accent);
  font-weight: 700;
  text-align: center;
}

.quota-field {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 110px;
  gap: 12px;
}

.quota-field__input,
.quota-field__unit {
  width: 100%;
}

.quota-panel {
  display: grid;
  gap: 10px;
  padding: 16px 18px;
  border-radius: 20px;
  border: 1px solid rgba(129, 145, 178, 0.16);
  background: rgba(255, 255, 255, 0.76);
}

.quota-panel__headline {
  font-size: 18px;
  font-weight: 700;
  color: var(--node-ink);
}

.quota-panel__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 13px;
  color: var(--node-muted);
}

.quota-policy-list {
  display: grid;
  gap: 10px;
}

.quota-policy-item {
  display: grid;
  gap: 6px;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(42, 102, 223, 0.06);
  border: 1px solid rgba(42, 102, 223, 0.1);
}

.quota-policy-item strong {
  color: var(--node-ink);
  font-size: 13px;
}

.quota-policy-item span {
  color: var(--node-muted);
  font-size: 13px;
  line-height: 1.6;
}

.option-secondary {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
}

.certificate-tip {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--node-accent);
}

.tags-input {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.tag-chip {
  margin: 0;
}

.tag-editor {
  width: 140px;
}

.switch-field {
  display: grid;
  gap: 10px;
}

.section-note {
  margin-bottom: 18px;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(42, 102, 223, 0.06);
  color: var(--node-muted);
  font-size: 13px;
  line-height: 1.6;
}

.section-actions {
  display: flex;
  justify-content: flex-start;
}

.summary-card__head {
  display: grid;
  gap: 10px;
}

.summary-card__eyebrow {
  font-size: 11px;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: var(--node-muted);
}

.summary-card__title {
  font-size: 28px;
  line-height: 1.15;
  color: var(--node-ink);
}

.summary-card__description {
  margin: 0;
  color: var(--node-muted);
  font-size: 13px;
  line-height: 1.7;
}

.summary-list {
  margin-top: 20px;
  display: grid;
}

.summary-list__item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding: 12px 0;
  border-top: 1px solid rgba(129, 145, 178, 0.14);
  font-size: 13px;
  color: var(--node-muted);
}

.summary-list__item strong {
  color: var(--node-ink);
  text-align: right;
  line-height: 1.5;
}

.summary-highlight {
  margin-top: 20px;
  padding: 16px;
  border-radius: 20px;
  background: linear-gradient(
    135deg,
    rgba(42, 102, 223, 0.08),
    rgba(245, 158, 11, 0.08)
  );
}

.summary-highlight span {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 700;
  color: var(--node-ink);
}

.summary-highlight p {
  margin: 0;
  color: var(--node-muted);
  font-size: 13px;
  line-height: 1.7;
}

.action-card {
  background: linear-gradient(
    180deg,
    rgba(255, 255, 255, 0.96),
    rgba(248, 250, 255, 0.94)
  );
}

.action-card__title {
  font-size: 22px;
  font-weight: 700;
  color: var(--node-ink);
}

.action-card__text {
  margin: 10px 0 20px;
  color: var(--node-muted);
  font-size: 13px;
  line-height: 1.7;
}

.action-card__primary,
.action-card__secondary {
  width: 100%;
  height: 48px;
  border-radius: 18px;
}

.action-card__secondary {
  margin-top: 10px;
  margin-left: 0;
}

.token-display {
  margin-top: 20px;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 14px;
}

.token-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.token-value {
  display: flex;
  align-items: center;
  gap: 12px;
}

.token-value code {
  flex: 1;
  padding: 8px 12px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  font-family: monospace;
  word-break: break-all;
}

.agent-config {
  margin-top: 20px;
}

.config-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.config-code {
  padding: 12px;
  background: var(--el-fill-color-darker);
  border-radius: 10px;
  font-family: monospace;
  font-size: 12px;
  overflow-x: auto;
  white-space: pre;
}

@media (max-width: 1180px) {
  .form-layout {
    grid-template-columns: 1fr;
  }

  .form-side {
    position: static;
  }
}

@media (max-width: 768px) {
  .node-form-page {
    padding: var(--vp-page-padding);
  }

  .hero-panel {
    padding: 22px 20px;
    border-radius: 24px;
  }

  .hero-panel__nav {
    flex-direction: column;
    gap: 18px;
  }

  .hero-panel__stats {
    grid-template-columns: 1fr;
    width: 100%;
  }

  .page-title {
    font-size: clamp(32px, 12vw, 42px);
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .field-span-2 {
    grid-column: auto;
  }

  .name-field,
  .slider-field,
  .quota-field {
    flex-direction: column;
    align-items: stretch;
  }

  .quota-field {
    grid-template-columns: 1fr;
  }

  .slider-pill {
    width: 100%;
  }

  .section-card :deep(.el-card__header),
  .section-card :deep(.el-card__body),
  .summary-card :deep(.el-card__body) {
    padding-left: 18px;
    padding-right: 18px;
  }

  .address-field__controls {
    grid-template-columns: 1fr;
  }

  .address-type-switch {
    width: 100%;
  }
}
</style>
