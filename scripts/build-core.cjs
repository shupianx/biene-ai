const { mkdirSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const coreDir = path.join(rootDir, 'core')
const args = process.argv.slice(2)

function readOption(name) {
  const inlinePrefix = `--${name}=`
  const inline = args.find((arg) => arg.startsWith(inlinePrefix))
  if (inline) return inline.slice(inlinePrefix.length)

  const index = args.indexOf(`--${name}`)
  if (index >= 0 && index + 1 < args.length) {
    return args[index + 1]
  }

  return ''
}

function normalizePlatform(value) {
  if (!value) return process.platform === 'win32' ? 'windows' : process.platform
  if (value === 'win32') return 'windows'
  return value
}

function normalizeArch(value) {
  switch (value) {
    case '':
      return process.arch === 'x64' ? 'amd64' : process.arch
    case 'x64':
      return 'amd64'
    case 'ia32':
      return '386'
    default:
      return value
  }
}

const targetPlatform = normalizePlatform(readOption('platform'))
const targetArch = normalizeArch(readOption('arch'))
const binaryName = targetPlatform === 'windows' ? 'biene-core.exe' : 'biene-core'
const outputArg = readOption('output')
const outputPath = outputArg
  ? path.resolve(rootDir, outputArg)
  : path.join(coreDir, 'dist', binaryName)
const outputDir = path.dirname(outputPath)

mkdirSync(outputDir, { recursive: true })

const env = {
  ...process.env,
  GOOS: targetPlatform,
  GOARCH: targetArch,
}

if ((targetPlatform !== normalizePlatform('') || targetArch !== normalizeArch('')) && env.CGO_ENABLED == null) {
  env.CGO_ENABLED = '0'
}

const result = spawnSync('go', ['build', '-o', outputPath, '.'], {
  cwd: coreDir,
  stdio: 'inherit',
  env,
})

if (result.status !== 0) {
  process.exit(result.status ?? 1)
}
