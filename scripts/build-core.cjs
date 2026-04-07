const { mkdirSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const coreDir = path.join(rootDir, 'core')
const outputDir = path.join(coreDir, 'dist')
const binaryName = process.platform === 'win32' ? 'biene-core.exe' : 'biene-core'
const outputPath = path.join(outputDir, binaryName)

mkdirSync(outputDir, { recursive: true })

const result = spawnSync('go', ['build', '-o', outputPath, '.'], {
  cwd: coreDir,
  stdio: 'inherit',
  env: process.env,
})

if (result.status !== 0) {
  process.exit(result.status ?? 1)
}
