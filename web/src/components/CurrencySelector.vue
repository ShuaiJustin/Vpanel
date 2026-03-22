<template>
  <el-dropdown
    trigger="click"
    @command="handleCurrencyChange"
  >
    <span class="currency-selector">
      <span class="currency-symbol">{{ currencySymbol }}</span>
      <span class="currency-code">{{ currentCurrency }}</span>
      <el-icon class="el-icon--right"><arrow-down /></el-icon>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="currency in currencies"
          :key="currency.code"
          :command="currency.code"
          :class="{ 'is-active': currency.code === currentCurrency }"
        >
          <span class="currency-item">
            <span class="currency-item-symbol">{{ currency.symbol }}</span>
            <span class="currency-item-code">{{ currency.code }}</span>
            <span class="currency-item-name">{{ currency.name }}</span>
          </span>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup>
import { computed } from 'vue'
import { ArrowDown } from '@element-plus/icons-vue'
import { useCurrencyStore } from '@/stores/currency'

const currencyStore = useCurrencyStore()

const currencies = computed(() => currencyStore.currencies)
const currentCurrency = computed(() => currencyStore.currentCurrency)
const currencySymbol = computed(() => currencyStore.currencySymbol)

function handleCurrencyChange(currency) {
  currencyStore.setCurrency(currency)
}
</script>

<style scoped>
.currency-selector {
  display: inline-flex;
  align-items: center;
  cursor: pointer;
  padding: 6px 12px;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.currency-selector:hover {
  background-color: var(--el-fill-color-light);
}

.currency-symbol {
  font-weight: 600;
  margin-right: 4px;
}

.currency-code {
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.currency-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 180px;
}

.currency-item-symbol {
  width: 24px;
  font-weight: 600;
  text-align: center;
}

.currency-item-code {
  width: 40px;
  font-weight: 500;
}

.currency-item-name {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.is-active {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}
</style>
