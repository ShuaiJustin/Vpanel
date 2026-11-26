/**
 * Centralized ECharts configuration with selective imports
 * 
 * This file imports only the chart types and components actually used in the application,
 * reducing the ECharts bundle size from ~800KB to ~200KB.
 * 
 * Used chart types: LineChart, BarChart, PieChart
 * Used components: Title, Tooltip, Grid, Legend
 * Renderer: CanvasRenderer
 */

import * as echarts from 'echarts/core'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

// Register only the components we use
echarts.use([
  LineChart,
  BarChart,
  PieChart,
  TitleComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
  CanvasRenderer,
])

export default echarts
