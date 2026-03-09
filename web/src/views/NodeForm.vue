<template>
  <div class="node-form-page">
    <div class="page-header">
      <div class="header-left">
        <el-button link @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h1 class="page-title">{{ isEdit ? "编辑节点" : "添加节点" }}</h1>
      </div>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
        style="max-width: 600px"
      >
        <el-form-item label="节点名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入节点名称" />
        </el-form-item>

        <el-form-item label="节点地址" prop="address">
          <el-input v-model="form.address" placeholder="IP 地址或域名">
            <template #prepend>
              <el-select v-model="addressType" style="width: 80px">
                <el-option label="IP" value="ip" />
                <el-option label="域名" value="domain" />
              </el-select>
            </template>
          </el-input>
        </el-form-item>

        <el-form-item label="Agent 端口" prop="port">
          <el-input-number
            v-model="form.port"
            :min="1"
            :max="65535"
            style="width: 200px"
          />
          <span class="form-tip">Node Agent 监听端口</span>
        </el-form-item>

        <el-form-item label="地区" prop="region">
          <el-select
            v-model="form.region"
            filterable
            allow-create
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
        </el-form-item>

        <el-form-item label="负载均衡权重" prop="weight">
          <el-slider
            v-model="form.weight"
            :min="1"
            :max="100"
            show-input
            style="width: 400px"
          />
          <div class="form-tip">权重越高，分配的用户越多</div>
        </el-form-item>

        <el-form-item label="最大用户数" prop="max_users">
          <el-input-number
            v-model="form.max_users"
            :min="0"
            style="width: 200px"
          />
          <span class="form-tip">0 表示无限制</span>
        </el-form-item>

        <el-form-item label="标签">
          <div class="tags-input">
            <el-tag
              v-for="(tag, index) in form.tags"
              :key="index"
              closable
              @close="removeTag(index)"
              style="margin-right: 8px; margin-bottom: 8px"
            >
              {{ tag }}
            </el-tag>
            <el-input
              v-if="showTagInput"
              ref="tagInputRef"
              v-model="newTag"
              size="small"
              style="width: 120px"
              @keyup.enter="addTag"
              @blur="addTag"
            />
            <el-button v-else size="small" @click="showTagInputField"
              >+ 添加标签</el-button
            >
          </div>
        </el-form-item>

        <el-form-item label="IP 白名单">
          <el-input
            v-model="form.ip_whitelist_str"
            type="textarea"
            :rows="4"
            placeholder="每行一个 IP 地址，留空表示不限制&#10;支持 CIDR 格式，如 192.168.1.0/24"
          />
          <div class="form-tip">限制可以连接到此节点的 IP 地址</div>
        </el-form-item>

        <el-divider />

        <el-form-item label="所属分组">
          <el-select
            v-model="form.group_ids"
            multiple
            filterable
            placeholder="选择分组（可多选）"
            style="width: 100%"
          >
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name"
              :value="group.id"
            >
              <span>{{ group.name }}</span>
              <span
                style="color: var(--el-text-color-secondary); margin-left: 8px"
              >
                {{ group.region }}
              </span>
            </el-option>
          </el-select>
          <div class="form-tip">节点可以同时属于多个分组</div>
        </el-form-item>

        <el-divider />

        <el-form-item label="启用 TLS">
          <el-switch
            v-model="form.tls_enabled"
            active-text="开启"
            inactive-text="关闭"
          />
        </el-form-item>

        <el-form-item label="TLS 域名" prop="tls_domain">
          <el-input
            v-model="form.tls_domain"
            placeholder="如 jp.example.com"
          />
          <div class="form-tip-inline">用于节点 TLS 标识、健康检查和系统证书自动匹配。</div>
        </el-form-item>

        <el-form-item label="系统证书">
          <el-select
            v-model="form.certificate_id"
            filterable
            clearable
            :loading="certificatesLoading"
            placeholder="自动匹配或手动选择证书"
            style="width: 100%"
            @change="handleCertificateChange"
          >
            <el-option
              v-for="cert in certificates"
              :key="cert.id"
              :label="getCertificateOptionLabel(cert)"
              :value="cert.id"
            />
          </el-select>
          <div class="form-tip-inline">选择后会自动回填 TLS 域名，你仍可继续手动修改。</div>
          <div v-if="selectedCertificate" class="certificate-tip">
            当前证书：{{ selectedCertificate.domain }}
            <span v-if="selectedCertificate.expireDate && selectedCertificate.expireDate !== '-'">
              ，到期 {{ selectedCertificate.expireDate }}
            </span>
          </div>
        </el-form-item>

        <!-- SSH 自动安装（仅新建时显示） -->
        <template v-if="!isEdit">
          <el-divider />

          <div style="margin-bottom: 16px">
            <el-checkbox v-model="enableAutoInstall">
              自动安装 Agent（通过 SSH 远程安装）
            </el-checkbox>
            <div class="form-tip" style="margin-left: 0; margin-top: 8px">
              勾选后，系统将自动连接到服务器并安装 Agent 和 Xray
            </div>
          </div>

          <template v-if="enableAutoInstall">
            <el-form-item label="服务器 IP">
              <el-input v-model="form.ssh_host" placeholder="服务器 IP 地址" />
            </el-form-item>

            <el-form-item label="SSH 端口">
              <el-input-number
                v-model="form.ssh_port"
                :min="1"
                :max="65535"
                style="width: 200px"
              />
            </el-form-item>

            <el-form-item label="SSH 用户名">
              <el-input v-model="form.ssh_username" placeholder="通常为 root" />
            </el-form-item>

            <el-form-item label="认证方式">
              <el-radio-group v-model="form.ssh_auth_type">
                <el-radio value="password">密码</el-radio>
                <el-radio value="key">私钥</el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item
              v-if="form.ssh_auth_type === 'password'"
              label="SSH 密码"
            >
              <el-input
                v-model="form.ssh_password"
                type="password"
                placeholder="SSH 登录密码"
                show-password
              />
            </el-form-item>

            <el-form-item v-if="form.ssh_auth_type === 'key'" label="SSH 私钥">
              <el-input
                v-model="form.ssh_private_key"
                type="textarea"
                :rows="6"
                placeholder="粘贴 SSH 私钥内容"
              />
            </el-form-item>

            <el-form-item>
              <el-button
                @click="testSSHConnection"
                :loading="testingConnection"
              >
                测试 SSH 连接
              </el-button>
            </el-form-item>
          </template>
        </template>

        <el-form-item>
          <el-button type="primary" @click="submitForm" :loading="submitting">
            {{
              isEdit
                ? "保存修改"
                : enableAutoInstall
                  ? "创建并安装"
                  : "创建节点"
            }}
          </el-button>
          <el-button @click="goBack">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 创建成功后显示 Token -->
    <el-dialog
      v-model="tokenDialogVisible"
      title="节点创建成功"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-alert type="success" :closable="false" show-icon>
        <template #title>节点已创建成功！</template>
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
        <el-button type="primary" @click="finishCreate"
          >我已保存，完成</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { ArrowLeft, CopyDocument } from "@element-plus/icons-vue";
import { useNodeStore } from "@/stores/node";
import { certificatesApi, nodeGroupsApi, nodesApi } from "@/api";

const route = useRoute();
const router = useRouter();
const nodeStore = useNodeStore();

const isEdit = computed(() => !!route.params.id);
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

// SSH 自动安装相关
const enableAutoInstall = ref(false);
const testingConnection = ref(false);

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
  tls_enabled: false,
  tls_domain: "",
  certificate_id: null,
  // SSH 连接信息
  ssh_host: "",
  ssh_port: 22,
  ssh_username: "root",
  ssh_auth_type: "password",
  ssh_password: "",
  ssh_private_key: "",
});

const rules = {
  name: [
    { required: true, message: "请输入节点名称", trigger: "blur" },
    { min: 2, max: 64, message: "名称长度在 2 到 64 个字符", trigger: "blur" },
  ],
  address: [
    { required: true, message: "请输入节点地址", trigger: "blur" },
    { validator: validateAddress, trigger: "blur" },
  ],
  port: [{ required: true, message: "请输入端口", trigger: "blur" }],
  tls_domain: [{ validator: validateTLSDomain, trigger: "blur" }],
};

function validateAddress(rule, value, callback) {
  if (!value) {
    callback(new Error("请输入节点地址"));
    return;
  }

  if (addressType.value === "ip") {
    // IPv4 或 IPv6 验证
    const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/;
    const ipv6Regex =
      /^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$|^::$|^([0-9a-fA-F]{1,4}:)*::([0-9a-fA-F]{1,4}:)*[0-9a-fA-F]{1,4}$/;
    if (!ipv4Regex.test(value) && !ipv6Regex.test(value)) {
      callback(new Error("请输入有效的 IP 地址"));
      return;
    }
  } else {
    // 域名验证
    const domainRegex =
      /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/;
    if (!domainRegex.test(value)) {
      callback(new Error("请输入有效的域名"));
      return;
    }
  }
  callback();
}


const normalizeText = (value) =>
  typeof value === "string" ? value.trim() : "";

const normalizeCertificateDomain = (domain) =>
  normalizeText(domain).replace(/^\*\./, "").toLowerCase();

const formatCertificateDate = (value) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toISOString().slice(0, 10);
};

const normalizeCertificatesResponse = (response) => {
  if (Array.isArray(response)) return response;
  if (Array.isArray(response?.certificates)) return response.certificates;
  if (Array.isArray(response?.data?.certificates)) return response.data.certificates;
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
  address: "${window.location.origin}"
  token: "${createdToken.value}"
  
node:
  name: "${form.name}"
  
xray:
  config_path: "/etc/xray/config.json"`;
});

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
    certificates.value = normalizeCertificatesResponse(response).map((cert) => ({
      ...cert,
      expireDate: formatCertificateDate(cert.expires_at || cert.expiresAt),
    }));
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
    form.group_ids = node.group_ids || [];
    form.tls_enabled = Boolean(node.tls_enabled);
    form.tls_domain = node.tls_domain || "";
    form.certificate_id = node.certificate_id ?? null;

    // 判断地址类型
    const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$|^([0-9a-fA-F]{1,4}:)/;
    addressType.value = ipRegex.test(node.address) ? "ip" : "domain";
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
    } catch {
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
  await formRef.value.validate();

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
      group_ids: form.group_ids,
      tls_enabled: form.tls_enabled,
      tls_domain: normalizeText(form.tls_domain).toLowerCase(),
      certificate_id: form.certificate_id || null,
    };

    // 如果开启了自动安装，添加 SSH 信息
    if (enableAutoInstall.value) {
      if (!form.ssh_host) {
        ElMessage.error("请输入服务器 IP");
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
        host: form.ssh_host,
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
    ElMessage.error(e.message || "操作失败");
  } finally {
    submitting.value = false;
  }
};

const copyToken = async () => {
  try {
    await navigator.clipboard.writeText(createdToken.value);
    ElMessage.success("已复制到剪贴板");
  } catch {
    ElMessage.error("复制失败");
  }
};

// 测试 SSH 连接
const testSSHConnection = async () => {
  if (!form.ssh_host) {
    ElMessage.error("请输入服务器 IP");
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
      host: form.ssh_host,
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

onMounted(async () => {
  await Promise.all([fetchGroups(), fetchCertificates()]);
  await fetchNode();
});
</script>

<style scoped>
.node-form-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-left: 12px;
}

.form-tip-inline {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 6px;
  line-height: 1.5;
}

.certificate-tip {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-color-primary);
}

.tags-input {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
}

.token-display {
  margin-top: 20px;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
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
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  overflow-x: auto;
  white-space: pre;
}
</style>
