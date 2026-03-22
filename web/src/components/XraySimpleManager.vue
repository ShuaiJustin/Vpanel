<template>
  <div class="xray-version-switcher">
    <div class="version-info">
      <div class="current-version">
        <span class="label">当前版本:</span>
        <el-tag
          size="large"
          type="success"
        >
          {{ currentVersion }}
        </el-tag>
      </div>

      <div class="version-selector">
        <el-select
          v-model="selectedVersion"
          placeholder="选择版本"
          size="large"
          style="width: 160px"
        >
          <el-option
            v-for="version in versions"
            :key="version"
            :label="version"
            :value="version"
          />
        </el-select>
        <el-button
          type="primary"
          :disabled="currentVersion === selectedVersion"
          :loading="switching"
          @click="switchVersion"
        >
          切换版本
        </el-button>
      </div>
    </div>

    <div class="controls">
      <el-button
        type="success"
        icon="Refresh"
        :loading="restarting"
        @click="restartService"
      >
        重启服务
      </el-button>
      <el-checkbox
        v-model="autoUpdate"
        @change="updateSettings"
      >
        自动更新
      </el-checkbox>
    </div>
  </div>
</template>

<script>
import { defineComponent, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/index'

export default defineComponent({
  name: 'XraySimpleManager',
  setup() {
    const currentVersion = ref('unknown')
    const selectedVersion = ref('')
    const versions = ref([])
    const autoUpdate = ref(false)
    const switching = ref(false)
    const restarting = ref(false)

    const fetchVersions = async () => {
      try {
        const data = await api.get('/xray/versions')
        versions.value = data.versions || []
        currentVersion.value = data.currentVersion || versions.value[0] || 'unknown'
        selectedVersion.value = currentVersion.value
      } catch (error) {
        console.error('获取版本列表失败:', error)
        ElMessage.error('获取版本列表失败')
      }
    }

    const fetchSettings = async () => {
      try {
        const data = await api.get('/settings/xray')
        autoUpdate.value = Boolean(data.auto_update)
      } catch (error) {
        console.error('获取 Xray 设置失败:', error)
      }
    }

    const switchVersion = async () => {
      if (!selectedVersion.value || selectedVersion.value === currentVersion.value) {
        return
      }

      switching.value = true
      try {
        await api.post('/xray/switch-version', { version: selectedVersion.value })
        currentVersion.value = selectedVersion.value
        ElMessage.success(`已切换到 ${selectedVersion.value}`)
      } catch (error) {
        console.error('版本切换失败:', error)
        ElMessage.error('版本切换失败')
      } finally {
        switching.value = false
      }
    }

    const restartService = async () => {
      restarting.value = true
      try {
        await api.post('/xray/restart')
        ElMessage.success('服务已重启')
      } catch (error) {
        console.error('重启服务失败:', error)
        ElMessage.error('重启服务失败')
      } finally {
        restarting.value = false
      }
    }

    const updateSettings = async () => {
      try {
        await api.post('/settings/xray', { auto_update: autoUpdate.value })
        ElMessage.success(`自动更新已${autoUpdate.value ? '启用' : '禁用'}`)
      } catch (error) {
        console.error('更新设置失败:', error)
        ElMessage.error('更新设置失败')
      }
    }

    onMounted(async () => {
      await Promise.all([fetchVersions(), fetchSettings()])
    })

    return {
      autoUpdate,
      currentVersion,
      restarting,
      restartService,
      selectedVersion,
      switching,
      switchVersion,
      updateSettings,
      versions,
    }
  },
})
</script>

<style scoped>
.xray-version-switcher {
  max-width: 600px;
  margin: 0 auto;
  padding: 20px;
  border-radius: 8px;
  background-color: #f9fafc;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.05);
}

.version-info {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 20px;
}

.current-version {
  display: flex;
  align-items: center;
  gap: 10px;
}

.version-selector {
  display: flex;
  align-items: center;
  gap: 10px;
}

.controls {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #ebeef5;
}

.label {
  min-width: 80px;
  font-weight: 500;
}

@media (min-width: 768px) {
  .version-info {
    flex-direction: row;
    justify-content: space-between;
  }
}
</style>
