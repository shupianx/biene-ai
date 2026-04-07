const { spawnSync } = require('child_process')
const path = require('path')

const rootDir = path.resolve(__dirname, '..')

function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: rootDir,
    stdio: 'inherit',
    env: process.env,
  })

  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
}

run('npm', ['--prefix', 'renderer', 'run', 'build'])
run(process.execPath, [path.join(rootDir, 'scripts', 'build-core.cjs')])
run('npm', ['run', 'package:desktop'])
