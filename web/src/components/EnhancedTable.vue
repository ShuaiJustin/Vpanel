<template>
  <div class="enhanced-table">
    <!-- 工具栏 -->
    <div
      v-if="showToolbar"
      class="table-toolbar"
    >
      <div class="toolbar-left">
        <slot name="toolbar-left">
          <el-input
            v-if="searchable"
            v-model="searchText"
            :placeholder="searchPlaceholder"
            clearable
            style="width: 240px"
            @input="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </slot>
      </div>
      <div class="toolbar-right">
        <slot name="toolbar-right">
          <el-button-group v-if="showColumnToggle">
            <el-popover
              placement="bottom"
              :width="200"
              trigger="click"
            >
              <template #reference>
                <el-button>
                  <el-icon><Setting /></el-icon>
                  列设置
                </el-button>
              </template>
              <div class="column-toggle-list">
                <el-checkbox
                  v-for="col in toggleableColumns"
                  :key="col.prop"
                  v-model="col.visible"
                  @change="handleColumnToggle"
                >
                  {{ col.label }}
                </el-checkbox>
              </div>
            </el-popover>
          </el-button-group>
          <el-button
            v-if="exportable"
            @click="handleExport"
          >
            <el-icon><Download /></el-icon>
            导出
          </el-button>
          <el-button
            v-if="refreshable"
            @click="$emit('refresh')"
          >
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </slot>
      </div>
    </div>

    <!-- 表格 -->
    <el-table
      ref="tableRef"
      v-loading="loading"
      :data="filteredData"
      :height="height"
      :max-height="maxHeight"
      :stripe="stripe"
      :border="border"
      :row-key="rowKey"
      :default-sort="defaultSort"
      v-bind="$attrs"
      @sort-change="handleSortChange"
      @selection-change="handleSelectionChange"
    >
      <!-- 选择列 -->
      <el-table-column
        v-if="selectable"
        type="selection"
        width="55"
        align="center"
      />

      <!-- 序号列 -->
      <el-table-column
        v-if="showIndex"
        type="index"
        label="#"
        width="60"
        align="center"
      />

      <!-- 数据列 -->
      <el-table-column
        v-for="col in visibleColumns"
        :key="col.prop"
        :prop="col.prop"
        :label="col.label"
        :width="col.width"
        :min-width="col.minWidth"
        :sortable="col.sortable"
        :align="col.align || 'left'"
        :fixed="col.fixed"
        :show-overflow-tooltip="col.showOverflowTooltip !== false"
      >
        <template
          v-if="col.slot || col.formatter"
          #default="scope"
        >
          <slot
            :name="col.slot || col.prop"
            :row="scope.row"
            :index="scope.$index"
          >
            <span v-if="col.formatter">{{ col.formatter(scope.row, col) }}</span>
            <span v-else>{{ scope.row[col.prop] }}</span>
          </slot>
        </template>
      </el-table-column>

      <!-- 操作列 -->
      <el-table-column
        v-if="$slots.actions"
        label="操作"
        :width="actionsWidth"
        :fixed="actionsFixed"
        align="center"
      >
        <template #default="scope">
          <slot
            name="actions"
            :row="scope.row"
            :index="scope.$index"
          />
        </template>
      </el-table-column>

      <!-- 空状态 -->
      <template #empty>
        <slot name="empty">
          <EmptyState
            type="no-data"
            size="small"
          />
        </slot>
      </template>
    </el-table>

    <!-- 分页 -->
    <div
      v-if="pagination"
      class="table-pagination"
    >
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="pageSizes"
        :total="total"
        :layout="paginationLayout"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<script setup>
/**
 * 增强表格组件
 * 提供排序、筛选、分页、导出等功能
 */
import { ref, computed } from 'vue'
import { Search, Setting, Download, Refresh } from '@element-plus/icons-vue'
import EmptyState from './EmptyState.vue'
import { debounce } from '@/utils/debounce'

const props = defineProps({
  // 数据
  data: {
    type: Array,
    default: () => []
  },
  // 列配置
  columns: {
    type: Array,
    required: true
  },
  // 加载状态
  loading: {
    type: Boolean,
    default: false
  },
  // 高度
  height: {
    type: [String, Number],
    default: undefined
  },
  // 最大高度
  maxHeight: {
    type: [String, Number],
    default: undefined
  },
  // 斑马纹
  stripe: {
    type: Boolean,
    default: true
  },
  // 边框
  border: {
    type: Boolean,
    default: false
  },
  // 行键
  rowKey: {
    type: [String, Function],
    default: 'id'
  },
  // 默认排序
  defaultSort: {
    type: Object,
    default: () => ({})
  },
  // 是否可选择
  selectable: {
    type: Boolean,
    default: false
  },
  // 是否显示序号
  showIndex: {
    type: Boolean,
    default: false
  },
  // 是否可搜索
  searchable: {
    type: Boolean,
    default: false
  },
  // 搜索占位符
  searchPlaceholder: {
    type: String,
    default: '搜索...'
  },
  // 搜索字段
  searchFields: {
    type: Array,
    default: () => []
  },
  // 是否显示工具栏
  showToolbar: {
    type: Boolean,
    default: true
  },
  // 是否显示列切换
  showColumnToggle: {
    type: Boolean,
    default: false
  },
  // 是否可导出
  exportable: {
    type: Boolean,
    default: false
  },
  // 是否可刷新
  refreshable: {
    type: Boolean,
    default: false
  },
  // 操作列宽度
  actionsWidth: {
    type: [String, Number],
    default: 150
  },
  // 操作列固定
  actionsFixed: {
    type: [Boolean, String],
    default: 'right'
  },
  // 是否分页
  pagination: {
    type: Boolean,
    default: true
  },
  // 总数
  total: {
    type: Number,
    default: 0
  },
  // 分页大小选项
  pageSizes: {
    type: Array,
    default: () => [10, 20, 50, 100]
  },
  // 分页布局
  paginationLayout: {
    type: String,
    default: 'total, sizes, prev, pager, next, jumper'
  }
})

const emit = defineEmits([
  'sort-change',
  'selection-change',
  'page-change',
  'size-change',
  'search',
  'refresh',
  'export'
])

// 表格引用
const tableRef = ref(null)

// 搜索文本
const searchText = ref('')

// 当前页
const currentPage = ref(1)

// 每页大小
const pageSize = ref(props.pageSizes[0] || 10)

// 可切换的列
const toggleableColumns = ref(
  props.columns.map(col => ({
    ...col,
    visible: col.visible !== false
  }))
)

// 可见列
const visibleColumns = computed(() => 
  toggleableColumns.value.filter(col => col.visible)
)

// 过滤后的数据
const filteredData = computed(() => {
  if (!searchText.value || !props.searchable) {
    return props.data
  }
  
  const search = searchText.value.toLowerCase()
  const fields = props.searchFields.length > 0 
    ? props.searchFields 
    : props.columns.map(c => c.prop)
  
  return props.data.filter(row => 
    fields.some(field => {
      const value = row[field]
      return value && String(value).toLowerCase().includes(search)
    })
  )
})

// 处理搜索
const handleSearch = debounce((value) => {
  emit('search', value)
}, 300)

// 处理排序变化
const handleSortChange = ({ prop, order }) => {
  emit('sort-change', { prop, order })
}

// 处理选择变化
const handleSelectionChange = (selection) => {
  emit('selection-change', selection)
}

// 处理页码变化
const handlePageChange = (page) => {
  emit('page-change', page)
}

// 处理每页大小变化
const handleSizeChange = (size) => {
  emit('size-change', size)
}

// 处理列切换
const handleColumnToggle = () => {
  // 触发重新渲染
}

// 处理导出
const handleExport = () => {
  emit('export', filteredData.value)
}

// 暴露方法
defineExpose({
  clearSelection: () => tableRef.value?.clearSelection(),
  toggleRowSelection: (row, selected) => tableRef.value?.toggleRowSelection(row, selected),
  toggleAllSelection: () => tableRef.value?.toggleAllSelection(),
  setCurrentRow: (row) => tableRef.value?.setCurrentRow(row),
  clearSort: () => tableRef.value?.clearSort(),
  sort: (prop, order) => tableRef.value?.sort(prop, order)
})
</script>

<style scoped>
.enhanced-table {
  width: 100%;
}

.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.column-toggle-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.table-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
