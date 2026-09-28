import { spawn } from 'node:child_process'
import { mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import puppeteer from 'puppeteer-core'

const frontendDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repositoryDirectory = path.resolve(frontendDirectory, '..')
const outputDirectory = path.join(repositoryDirectory, 'docs', 'previews')
const port = 4173
const baseUrl = `http://127.0.0.1:${port}`
const executablePath = process.env.SILICON_PREVIEW_BROWSER || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'

async function waitForServer() {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    try {
      const response = await fetch(baseUrl)
      if (response.ok) return
    } catch {
      // The local preview server may still be starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 200))
  }
  throw new Error(`Preview server did not become ready at ${baseUrl}`)
}

await mkdir(outputDirectory, { recursive: true })
const server = spawn(process.execPath, ['./node_modules/vite/bin/vite.js', '--host', '127.0.0.1', '--port', String(port), '--strictPort'], {
  cwd: frontendDirectory,
  env: { ...process.env, VITE_PREVIEW_FIXTURES: 'true' },
  stdio: 'ignore',
})

let browser
try {
  await waitForServer()
  browser = await puppeteer.launch({ executablePath, headless: true, args: ['--disable-gpu'] })
  const page = await browser.newPage()
  await page.setViewport({ width: 1440, height: 1000, deviceScaleFactor: 1 })
  await page.goto(`${baseUrl}/docs/overview/getting-started`, { waitUntil: 'networkidle0' })
  await page.waitForSelector('.docs-article h1')
  await page.screenshot({ path: path.join(outputDirectory, 'documentation.png') })

  await page.keyboard.down(process.platform === 'darwin' ? 'Meta' : 'Control')
  await page.keyboard.press('KeyK')
  await page.keyboard.up(process.platform === 'darwin' ? 'Meta' : 'Control')
  await page.waitForSelector('.command-palette')
  await page.type('.command-input-row input', 'server')
  await new Promise((resolve) => setTimeout(resolve, 350))
  await page.screenshot({ path: path.join(outputDirectory, 'global-search.png') })
} finally {
  await browser?.close()
  server.kill()
}
