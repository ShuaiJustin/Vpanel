/**
 * Element Plus on-demand stylesheets.
 *
 * Generated from the components actually used under web/src (see
 * scripts/scan-element-plus.mjs). Keep this list in sync by running:
 *
 *   npm run scan:ep
 *
 * Why these imports instead of `element-plus/dist/index.css`:
 * - `dist/index.css` bundles all ~120 components (350 KB raw). Only 54
 *   components + 4 imperative services are used, so importing their
 *   individual style entries cuts the shipped CSS substantially.
 * - Each `es/components/<name>/style/css` entry declares its own
 *   dependencies (base.css, el-popper.css, el-overlay.css,
 *   el-scrollbar.css, el-tag.css, ...), so shared/dependent styles are
 *   still pulled in automatically and de-duplicated by Vite.
 * - `base.css` supplies the full `:root` CSS variable set. Project theme
 *   customization in styles/theme.css, admin-ui.css and dark-mode-fixes.css
 *   keeps working because those files override the same variables.
 * - `dark/css-vars.css` supplies the `html.dark` variable set; dark mode
 *   keeps working as before.
 *
 * DO NOT re-add `element-plus/dist/index.css` here.
 */

// Core variables + reset (explicit, so theme overrides always have a base)
import 'element-plus/theme-chalk/base.css'
// Dark mode variable set (html.dark)
import 'element-plus/theme-chalk/dark/css-vars.css'

// ---------------------------------------------------------------------------
// Components used in templates (54). Add entries ONLY when scan:ep reports a
// missing component; entries that are no longer used should be removed.
// ---------------------------------------------------------------------------
import 'element-plus/es/components/alert/style/css'
import 'element-plus/es/components/avatar/style/css'
import 'element-plus/es/components/badge/style/css'
import 'element-plus/es/components/button/style/css'
import 'element-plus/es/components/button-group/style/css'
import 'element-plus/es/components/card/style/css'
import 'element-plus/es/components/checkbox/style/css'
import 'element-plus/es/components/checkbox-group/style/css'
import 'element-plus/es/components/col/style/css'
import 'element-plus/es/components/collapse/style/css'
import 'element-plus/es/components/collapse-item/style/css'
import 'element-plus/es/components/date-picker/style/css'
import 'element-plus/es/components/descriptions/style/css'
import 'element-plus/es/components/descriptions-item/style/css'
import 'element-plus/es/components/dialog/style/css'
import 'element-plus/es/components/divider/style/css'
import 'element-plus/es/components/drawer/style/css'
import 'element-plus/es/components/dropdown/style/css'
import 'element-plus/es/components/dropdown-item/style/css'
import 'element-plus/es/components/dropdown-menu/style/css'
import 'element-plus/es/components/empty/style/css'
import 'element-plus/es/components/form/style/css'
import 'element-plus/es/components/form-item/style/css'
import 'element-plus/es/components/icon/style/css'
import 'element-plus/es/components/input/style/css'
import 'element-plus/es/components/input-number/style/css'
import 'element-plus/es/components/link/style/css'
import 'element-plus/es/components/menu/style/css'
import 'element-plus/es/components/menu-item/style/css'
import 'element-plus/es/components/option/style/css'
import 'element-plus/es/components/pagination/style/css'
import 'element-plus/es/components/popover/style/css'
import 'element-plus/es/components/progress/style/css'
import 'element-plus/es/components/radio/style/css'
import 'element-plus/es/components/radio-button/style/css'
import 'element-plus/es/components/radio-group/style/css'
import 'element-plus/es/components/row/style/css'
import 'element-plus/es/components/select/style/css'
import 'element-plus/es/components/skeleton/style/css'
import 'element-plus/es/components/slider/style/css'
import 'element-plus/es/components/step/style/css'
import 'element-plus/es/components/steps/style/css'
import 'element-plus/es/components/sub-menu/style/css'
import 'element-plus/es/components/switch/style/css'
import 'element-plus/es/components/tab-pane/style/css'
import 'element-plus/es/components/table/style/css'
import 'element-plus/es/components/table-column/style/css'
import 'element-plus/es/components/tabs/style/css'
import 'element-plus/es/components/tag/style/css'
import 'element-plus/es/components/timeline/style/css'
import 'element-plus/es/components/timeline-item/style/css'
import 'element-plus/es/components/tooltip/style/css'
import 'element-plus/es/components/transfer/style/css'
import 'element-plus/es/components/upload/style/css'

// ---------------------------------------------------------------------------
// Imperative / service APIs (not template tags) and the v-loading directive.
// These are the easiest ones to forget when switching to on-demand CSS.
// ---------------------------------------------------------------------------
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'
import 'element-plus/es/components/notification/style/css'
import 'element-plus/es/components/loading/style/css'
