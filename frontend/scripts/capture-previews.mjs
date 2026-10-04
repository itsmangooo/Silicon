import { spawn } from 'node:child_process'
import { mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import puppeteer from 'puppeteer-core'

const frontendDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repositoryDirectory = path.resolve(frontendDirectory, '..')
const outputDirectory = path.join(repositoryDirectory, 'docs', 'previews')
const guestPort = 4173
const previewPort = 4174
const guestUrl = `http://127.0.0.1:${guestPort}`
const previewUrl = `http://127.0.0.1:${previewPort}`
const executablePath = process.env.SILICON_PREVIEW_BROWSER || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'

async function waitForServer(baseUrl) {
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

function startServer(port, previewFixtures) {
  return spawn(process.execPath, ['./node_modules/vite/bin/vite.js', '--host', '127.0.0.1', '--port', String(port), '--strictPort'], {
    cwd: frontendDirectory,
    env: { ...process.env, VITE_PREVIEW_FIXTURES: String(previewFixtures) },
    stdio: 'ignore',
  })
}

async function settle(page, selector) {
  await page.waitForSelector(selector)
  await page.evaluate(() => globalThis.document.fonts.ready)
  await new Promise((resolve) => setTimeout(resolve, 100))
}

async function capture(page, baseUrl, route, filename, selector = '.section', fullPage = true) {
  await page.goto(`${baseUrl}${route}`, { waitUntil: 'networkidle0' })
  await settle(page, selector)
  await page.screenshot({ path: path.join(outputDirectory, filename), fullPage })
}

async function captureElement(page, filename, selector) {
  await settle(page, selector)
  const element = await page.$(selector)
  if (!element) throw new Error(`Could not find ${selector}`)
  await element.evaluate((node) => node.scrollIntoView({ block: 'center' }))
  await new Promise((resolve) => setTimeout(resolve, 100))
  await element.screenshot({ path: path.join(outputDirectory, filename) })
}

async function clickButton(page, label) {
  const clicked = await page.evaluate((text) => {
    const button = [...globalThis.document.querySelectorAll('button')].find((item) => item.textContent.trim().includes(text))
    button?.click()
    return Boolean(button)
  }, label)
  if (!clicked) throw new Error(`Could not find button containing ${label}`)
}

await mkdir(outputDirectory, { recursive: true })
const guestServer = startServer(guestPort, false)
const previewServer = startServer(previewPort, true)

let browser
try {
  await Promise.all([waitForServer(guestUrl), waitForServer(previewUrl)])
  browser = await puppeteer.launch({ executablePath, headless: true, args: ['--disable-gpu'] })

  const guestPage = await browser.newPage()
  await guestPage.setViewport({ width: 1440, height: 1000, deviceScaleFactor: 1 })
  await guestPage.setRequestInterception(true)
  guestPage.on('request', (request) => {
    if (request.url() === `${guestUrl}/api/v1/auth/session`) {
      request.respond({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: { message: 'Authentication required.' } }) })
      return
    }
    request.continue()
  })
  await capture(guestPage, guestUrl, '/login', 'login.png', '.auth-panel', false)
  await capture(guestPage, guestUrl, '/register', 'register.png', '.auth-panel', false)
  await capture(guestPage, guestUrl, '/forgot-password', 'forgot-password.png', '.auth-panel', false)
  await capture(guestPage, guestUrl, '/reset-password?token=sanitized-preview-token', 'reset-password.png', '.auth-panel', false)
  await guestPage.close()

  const page = await browser.newPage()
  await page.setViewport({ width: 1440, height: 1000, deviceScaleFactor: 1 })

  await capture(page, previewUrl, '/docs/overview/getting-started', 'documentation.png', '.docs-article')
  await page.keyboard.down(process.platform === 'darwin' ? 'Meta' : 'Control')
  await page.keyboard.press('KeyK')
  await page.keyboard.up(process.platform === 'darwin' ? 'Meta' : 'Control')
  await settle(page, '.command-palette')
  await page.type('.command-input-row input', 'server')
  await new Promise((resolve) => setTimeout(resolve, 350))
  await page.screenshot({ path: path.join(outputDirectory, 'global-search.png') })

  const previews = [
    ['/', 'dashboard.png'],
    ['/projects', 'projects.png'],
    ['/projects/project-platform', 'project-detail.png'],
    ['/environments', 'environments.png'],
    ['/projects/project-platform/environments/environment-production', 'environment-detail.png'],
    ['/applications', 'applications.png'],
    ['/projects/project-platform/applications/app-api', 'application-detail.png'],
    ['/deployments', 'deployments.png'],
    ['/deployments/deployment-184', 'deployment-detail.png'],
    ['/networks', 'networks.png'],
    ['/networks/network-production', 'network-detail.png'],
    ['/domains', 'domains.png'],
    ['/integrations', 'integrations.png'],
    ['/members', 'members.png'],
    ['/access', 'access.png'],
    ['/identity', 'identity-providers.png'],
    ['/audit', 'audit.png'],
    ['/settings', 'settings.png'],
  ]
  for (const [route, filename] of previews) await capture(page, previewUrl, route, filename)

  await capture(page, previewUrl, '/settings#updates', 'settings-updates.png', '.update-version-grid', false)
  await capture(page, previewUrl, '/settings', 'settings-public-access.png', '.section:first-of-type', false)
  await page.goto(`${previewUrl}/settings`, { waitUntil: 'networkidle0' })
  await page.addStyleTag({ content: '.topbar { display: none !important; }' })
  await captureElement(page, 'settings-email.png', '.section:nth-of-type(2)')

  await page.goto(`${previewUrl}/servers`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await page.screenshot({ path: path.join(outputDirectory, 'servers.png'), fullPage: true })
  await page.click('.table-link')
  await settle(page, '.dialog')
  await page.screenshot({ path: path.join(outputDirectory, 'server-ssh-detail.png') })

  await page.goto(`${previewUrl}/applications`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await clickButton(page, 'Create application')
  await settle(page, '.dialog')
  await page.select('select[name="sourceType"]', 'git_dockerfile')
  await page.type('input[name="internalPort"]', '3000')
  await page.type('input[name="publishedPort"]', '8080')
  await page.screenshot({ path: path.join(outputDirectory, 'application-create.png') })

  await page.setViewport({ width: 1440, height: 1200, deviceScaleFactor: 1 })
  await page.goto(`${previewUrl}/projects`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await clickButton(page, 'Create project')
  await page.type('input[name="name"]', 'example-app')
  await page.type('input[name="slug"]', 'example-app')
  await page.type('textarea[name="description"]', 'Frontend and backend production workloads')
  await captureElement(page, 'guide-create-project.png', '.dialog')

  await page.goto(`${previewUrl}/environments`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await clickButton(page, 'Create environment')
  await page.type('input[name="name"]', 'production')
  await page.type('input[name="slug"]', 'production')
  await captureElement(page, 'guide-create-environment.png', '.dialog')

  await page.goto(`${previewUrl}/servers`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await clickButton(page, 'Add server')
  await page.type('input[name="name"]', 'example-production')
  await page.select('select[name="connectivityType"]', 'self_hosted')
  await page.type('input[name="host"]', 'server.example.test')
  await page.type('input[name="username"]', 'silicon')
  await captureElement(page, 'guide-add-server.png', '.dialog')

  await page.goto(`${previewUrl}/servers`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await page.click('.table-link')
  await clickButton(page, 'Configure connection')
  await captureElement(page, 'guide-ssh-connection.png', '.dialog')

  await page.goto(`${previewUrl}/applications`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await clickButton(page, 'Create application')
  await page.type('input[name="name"]', 'frontend')
  await page.select('select[name="sourceType"]', 'git_dockerfile')
  await page.select('select[name="targetId"]', 'server-edge')
  await page.type('input[name="internalPort"]', '3000')
  await page.type('input[name="publishedPort"]', '8080')
  await captureElement(page, 'guide-create-application.png', '.dialog')

  await page.goto(`${previewUrl}/applications`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await clickButton(page, 'Configure')
  await captureElement(page, 'guide-deployment-target.png', '.dialog')

  await page.goto(`${previewUrl}/projects/project-platform/applications/app-api`, { waitUntil: 'networkidle0' })
  await captureElement(page, 'guide-environment-variables.png', '.configuration-editor .split-tables')
  await captureElement(page, 'guide-secrets.png', '.configuration-editor #secrets')
  await clickButton(page, 'Inspect runtime')
  await settle(page, '.dialog')
  await clickButton(page, 'Tail')
  await captureElement(page, 'guide-runtime-logs.png', '.dialog')

  await page.goto(`${previewUrl}/domains`, { waitUntil: 'networkidle0' })
  await page.type('input[name="hostname"]', 'api.example.com')
  await page.$eval('input[name="targetPort"]', (input) => {
    const inputElement = input.ownerDocument.defaultView.HTMLInputElement
    const setter = Object.getOwnPropertyDescriptor(inputElement.prototype, 'value').set
    setter.call(input, '18080')
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  await page.select('select[name="routingMode"]', 'cloudflare_tunnel')
  await captureElement(page, 'guide-domain-setup.png', '.section')

  await page.goto(`${previewUrl}/integrations`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await captureElement(page, 'guide-github-integration.png', '.section:nth-of-type(1)')
  await captureElement(page, 'guide-cloudflare-integration.png', '.section:nth-of-type(2)')

  await page.goto(`${previewUrl}/aws/accounts`, { waitUntil: 'networkidle0' })
  await settle(page, '.section')
  await clickButton(page, 'Connect account')
  await page.type('input[name="displayName"]', 'Production AWS')
  await page.type('input[name="accountId"]', '000000000000')
  await page.type('input[name="roleArn"]', 'arn:aws:iam::000000000000:role/Silicon')
  await captureElement(page, 'guide-aws-account.png', '.dialog')

  await capture(page, previewUrl, '/aws/compute', 'guide-aws-compute.png')
  await capture(page, previewUrl, '/aws/network', 'guide-aws-network.png')
} finally {
  await browser?.close()
  guestServer.kill()
  previewServer.kill()
}
