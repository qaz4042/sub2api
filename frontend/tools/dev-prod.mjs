#!/usr/bin/env node
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { spawn, spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { lookup } from 'node:dns/promises'
import { loadEnv } from 'vite'

const __dirname = dirname(fileURLToPath(import.meta.url))
const root = resolve(__dirname, '..')
const productionEnv = loadEnv('production', root, '')
const mergedEnv = { ...productionEnv, ...process.env }

const host = mergedEnv.VITE_DEV_HOST || 'local.portal.lizubin.online'
const port = mergedEnv.VITE_DEV_PORT || '3002'
const proxyTarget = mergedEnv.VITE_DEV_PROXY_TARGET || 'https://portal.lizubin.online'
const certDir = resolve(root, '.dev/certs')
const certPath = resolve(certDir, `${host}.crt`)
const keyPath = resolve(certDir, `${host}.key`)
const opensslConfigPath = resolve(certDir, `${host}.openssl.cnf`)
const viteBin = resolve(root, 'node_modules/.bin/vite')

function hasHostsEntry() {
  try {
    const hosts = readFileSync('/etc/hosts', 'utf8')
    return hosts.split(/\r?\n/).some((line) => {
      const normalized = line.replace(/#.*/, '').trim()
      return normalized.length > 0 && normalized.split(/\s+/).includes(host)
    })
  } catch {
    return false
  }
}

async function resolvesToLoopback() {
  try {
    const records = await lookup(host, { all: true })
    return records.some((record) => record.address === '127.0.0.1' || record.address === '::1')
  } catch {
    return false
  }
}

function ensureCertificate() {
  if (existsSync(certPath) && existsSync(keyPath)) {
    return
  }

  mkdirSync(certDir, { recursive: true })
  writeFileSync(
    opensslConfigPath,
    `[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
x509_extensions = v3_req

[dn]
CN = ${host}

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${host}
DNS.2 = localhost
IP.1 = 127.0.0.1
IP.2 = ::1
`
  )

  const result = spawnSync(
    'openssl',
    [
      'req',
      '-x509',
      '-newkey',
      'rsa:2048',
      '-sha256',
      '-days',
      '825',
      '-nodes',
      '-keyout',
      keyPath,
      '-out',
      certPath,
      '-config',
      opensslConfigPath
    ],
    { cwd: root, stdio: 'inherit' }
  )

  if (result.status !== 0) {
    process.exit(result.status || 1)
  }
}

async function main() {
  const hostsConfigured = hasHostsEntry() || (await resolvesToLoopback())
  if (!hostsConfigured) {
    console.warn(`
[dev:prod] ${host} 还没有解析到本机。
[dev:prod] 请先执行一次：

sudo sh -c 'grep -q "${host}" /etc/hosts || echo "127.0.0.1 ${host}" >> /etc/hosts'
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder

[dev:prod] 然后重新运行 dev:prod。
`)
    process.exit(1)
  }

  ensureCertificate()

  const env = {
    ...process.env,
    VITE_DEV_PROXY_TARGET: proxyTarget,
    VITE_DEV_HOST: host,
    VITE_DEV_PORT: port,
    VITE_DEV_HTTPS: 'true',
    VITE_DEV_HTTPS_CERT: certPath,
    VITE_DEV_HTTPS_KEY: keyPath
  }

  console.log(`[dev:prod] proxy target: ${proxyTarget}`)
  console.log(`[dev:prod] local url: https://${host}:${port}/login`)
  console.log('[dev:prod] 如果浏览器提示证书不受信任，请先信任本地开发证书。')

  const child = spawn(
    viteBin,
    ['--config', 'vite.config.ts', '--mode', 'production', '--host', '0.0.0.0', '--port', port, '--strictPort'],
    { cwd: root, env, stdio: 'inherit' }
  )

  child.on('exit', (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal)
      return
    }
    process.exit(code || 0)
  })
}

main().catch((error) => {
  console.error('[dev:prod] 启动失败:', error)
  process.exit(1)
})
