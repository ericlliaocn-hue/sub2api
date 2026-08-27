import { createHash } from 'node:crypto'
import { spawn } from 'node:child_process'
import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.dirname(fileURLToPath(import.meta.url))
const sourceEntries = ['build.mjs', 'watch.mjs', 'index.html', 'pages.json', 'styles.css', 'app.js', 'analytics.js', 'assets']

let buildRunning = false
let buildPending = false
let signatureCheckRunning = false
let sourceSignature = ''

async function collectFiles(entry) {
  const target = path.join(root, entry)
  const items = await readdir(target, { withFileTypes: true }).catch(() => null)
  if (!items) return [entry]

  const nested = await Promise.all(items.map((item) => {
    const relative = path.join(entry, item.name)
    return item.isDirectory() ? collectFiles(relative) : [relative]
  }))
  return nested.flat()
}

async function calculateSourceSignature() {
  const files = (await Promise.all(sourceEntries.map(collectFiles))).flat().sort()
  const hash = createHash('sha256')

  for (const file of files) {
    hash.update(file)
    hash.update(await readFile(path.join(root, file)))
  }

  return hash.digest('hex')
}

function runBuild() {
  if (buildRunning) {
    buildPending = true
    return
  }

  buildRunning = true
  const child = spawn(process.execPath, ['build.mjs'], {
    cwd: root,
    stdio: 'inherit',
  })

  child.on('exit', (code, signal) => {
    buildRunning = false
    if (signal) {
      console.error(`Documentation build terminated by ${signal}`)
    } else if (code !== 0) {
      console.error(`Documentation build failed with exit code ${code}`)
    }

    if (buildPending) {
      buildPending = false
      runBuild()
    }
  })
}

async function checkForChanges() {
  if (signatureCheckRunning) return
  signatureCheckRunning = true

  try {
    const nextSignature = await calculateSourceSignature()
    if (nextSignature !== sourceSignature) {
      sourceSignature = nextSignature
      console.log('Documentation source content changed')
      runBuild()
    }
  } catch (error) {
    console.error('Unable to check documentation sources:', error)
  } finally {
    signatureCheckRunning = false
  }
}

sourceSignature = await calculateSourceSignature()
runBuild()
const interval = setInterval(checkForChanges, 500)

function shutdown() {
  clearInterval(interval)
  process.exit(0)
}

process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)

console.log(`Watching ${sourceEntries.length} documentation source entries for content changes`)
