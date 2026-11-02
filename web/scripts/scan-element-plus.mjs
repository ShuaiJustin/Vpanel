#!/usr/bin/env node
/**
 * Verify that web/src/element-plus-styles.js matches the Element Plus
 * components actually used in the application source.
 *
 * Usage:  npm run scan:ep
 * Exit 1 when a used component has no stylesheet entry (or when an entry
 * points at a component that does not exist in the installed element-plus).
 *
 * This is the maintenance guard for the on-demand CSS migration: whenever a
 * new <el-xxx> tag or service API is introduced, `npm run scan:ep` tells you
 * exactly which import to add.
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const srcDir = path.join(webRoot, 'src')
const styleFile = path.join(srcDir, 'element-plus-styles.js')
const epRoot = path.join(webRoot, 'node_modules', 'element-plus')

const serviceMap = {
  ElMessage: 'message',
  ElMessageBox: 'message-box',
  ElNotification: 'notification',
  ElLoading: 'loading',
}

const isSourceFile = (file) => {
  const name = path.basename(file)
  if (!/\.(vue|js)$/.test(name)) return false
  if (/\.(test|spec)\.js$/.test(name)) return false
  if (file.includes(`${path.sep}tests${path.sep}`) || file.includes('/tests/')) return false
  return true
}

const walk = (dir, acc = []) => {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, acc)
    else if (entry.isFile() && isSourceFile(full)) acc.push(full)
  }
  return acc
}

const files = walk(srcDir)
const used = new Map() // component name -> Set of files

const mark = (name, file) => {
  if (!used.has(name)) used.set(name, new Set())
  used.get(name).add(path.relative(webRoot, file))
}

for (const file of files) {
  const text = fs.readFileSync(file, 'utf8')

  // Template tags: <el-button>, <el-table-column>, ...
  for (const match of text.matchAll(/<(el-[a-z0-9-]+)/g)) {
    mark(match[1].slice(3), file)
  }

  // Imperative services: ElMessage / ElMessageBox / ElNotification / ElLoading
  for (const match of text.matchAll(/\b(ElMessage|ElMessageBox|ElNotification|ElLoading)\b/g)) {
    mark(serviceMap[match[1]], file)
  }

  // v-loading directive
  if (/\bv-loading\b/.test(text)) mark('loading', file)
}

const styleSource = fs.readFileSync(styleFile, 'utf8')
const imported = new Set()
for (const match of styleSource.matchAll(/es\/components\/([a-z0-9-]+)\/style\/css/g)) {
  imported.add(match[1])
}

const usedNames = [...used.keys()].sort()
const importedNames = [...imported].sort()
const missing = usedNames.filter((name) => !imported.has(name))
const extra = importedNames.filter((name) => !used.has(name))
const brokenEntries = importedNames.filter(
  (name) => !fs.existsSync(path.join(epRoot, 'es', 'components', name, 'style', 'css.mjs'))
)

console.log(`Scanned files : ${files.length}`)
console.log(`Used parts    : ${usedNames.length}`)
console.log(`Imported parts: ${importedNames.length}`)
console.log(`Used          : ${usedNames.join(', ')}`)

if (extra.length) {
  console.log(`\n[warn] imported but not referenced (${extra.length}): ${extra.join(', ')}`)
  console.log('       Keep them only if they are required indirectly (e.g. dependent styles).')
}

if (brokenEntries.length) {
  console.error(`\n[error] style entries not found in installed element-plus: ${brokenEntries.join(', ')}`)
}

if (missing.length) {
  console.error(`\n[error] missing on-demand style entries (${missing.length}):`)
  for (const name of missing) {
    const where = [...used.get(name)].slice(0, 3).join(', ')
    console.error(`  - ${name.padEnd(20)} used in ${where}`)
    console.error(`    add: import 'element-plus/es/components/${name}/style/css'`)
  }
}

if (missing.length || brokenEntries.length) process.exit(1)
console.log('\n[ok] on-demand stylesheet list covers every used Element Plus part.')
