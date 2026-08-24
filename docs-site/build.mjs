import { cp, mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.dirname(fileURLToPath(import.meta.url))
const output = path.join(root, 'dist')
const origin = 'https://doc.anytoken.work'

const pages = [
  { id: 'start', route: '/', title: 'AnyToken API 文档', description: 'AnyToken API 接入文档：接口地址、身份验证、SDK、开发工具与错误排查。', group: '开始' },
  { id: 'quickstart', route: '/quickstart/', title: '快速开始', description: '创建 AnyToken API Key，并通过 Responses API 完成第一次模型请求。', group: '开始' },
  { id: 'concepts', route: '/guides/endpoints/', title: 'API 地址与路径', description: '了解 AnyToken 官网、OpenAI 兼容 API、Anthropic 兼容接口与版本路径的区别。', group: '接入指南' },
  { id: 'authentication', route: '/guides/authentication/', title: 'API 身份验证', description: '使用 Bearer API Key 或兼容请求头安全访问 AnyToken 模型 API。', group: '接入指南' },
  { id: 'responses', route: '/api/responses/', title: 'Responses API', description: '使用 AnyToken OpenAI 兼容 Responses API 发送文本和工具调用请求。', group: 'API 调用' },
  { id: 'chat', route: '/api/chat-completions/', title: 'Chat Completions API', description: '通过 AnyToken 调用兼容 OpenAI Chat Completions 的 messages 接口。', group: 'API 调用' },
  { id: 'models', route: '/api/models/', title: '查询可用模型', description: '使用 API Key 查询当前分组可以访问的 AnyToken 模型列表。', group: 'API 调用' },
  { id: 'streaming', route: '/api/streaming/', title: '流式响应', description: '通过 SSE 流式读取 AnyToken 模型 API 的实时输出。', group: 'API 调用' },
  { id: 'sdk', route: '/sdks/openai/', title: 'OpenAI SDK 接入', description: '使用 Python 和 Node.js OpenAI SDK 连接 AnyToken API。', group: '开发工具' },
  { id: 'codex', route: '/tools/codex-cli/', title: 'Codex CLI 配置', description: '配置 Codex CLI 通过 AnyToken OpenAI 兼容接口调用模型。', group: '开发工具' },
  { id: 'claude-code', route: '/tools/claude-code/', title: 'Claude Code 配置', description: '配置 Claude Code 使用 AnyToken Anthropic 兼容接口。', group: '开发工具' },
  { id: 'gemini-cli', route: '/tools/gemini-cli/', title: 'Gemini CLI 配置', description: '配置 Gemini CLI 使用 AnyToken 兼容接口和 API Key。', group: '开发工具' },
  { id: 'opencode', route: '/tools/opencode/', title: 'OpenCode 配置', description: '配置 OpenCode 通过 AnyToken 调用 OpenAI 兼容模型。', group: '开发工具' },
  { id: 'billing', route: '/account/billing/', title: '用量与计费', description: '查看 AnyToken 请求 Token、费用、倍率和使用记录。', group: '账户与排错' },
  { id: 'errors', route: '/troubleshooting/errors/', title: 'API 错误排查', description: '排查 AnyToken API 常见的鉴权、模型、限流、服务和 DNS 错误。', group: '账户与排错' },
  { id: 'security', route: '/security/api-keys/', title: 'API Key 安全', description: '安全保存、使用、审计和撤销 AnyToken API Key。', group: '账户与排错' },
  { id: 'faq', route: '/faq/', title: '常见问题', description: 'AnyToken API 地址、版本路径、模型列表和流式输出常见问题。', group: '账户与排错' },
]

const source = await readFile(path.join(root, 'index.html'), 'utf8')

function extract(pattern, label) {
  const match = source.match(pattern)
  if (!match) throw new Error(`Unable to extract ${label}`)
  return match[0]
}

const header = extract(/<header class="topbar">[\s\S]*?<\/header>/, 'header')
const sidebar = extract(/<aside class="sidebar"[\s\S]*?<\/aside>/, 'sidebar')
const footer = extract(/<footer class="doc-footer">[\s\S]*?<\/footer>/, 'footer')
const overlays = extract(/<div class="search-modal"[\s\S]*?<script src="\.\/app\.js" defer><\/script>/, 'search UI')

const sections = new Map()
for (const page of pages) {
  const pattern = new RegExp(`<section id="${page.id}"[\\s\\S]*?<\\/section>`)
  sections.set(page.id, extract(pattern, `section ${page.id}`))
}

function escapeHTML(value) {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;')
}

function absoluteAssets(html) {
  return html
    .replaceAll('href="./assets/', 'href="/assets/')
    .replaceAll('href="./styles.css"', 'href="/styles.css"')
    .replaceAll('src="./app.js"', 'src="/app.js"')
}

function routeNavigation(html, activeId) {
  let result = html
  for (const page of pages) {
    const active = page.id === activeId ? ' active' : ''
    const current = page.id === activeId ? ' aria-current="page"' : ''
    result = result.replaceAll(
      `href="#${page.id}" class="nav-link${page.id === 'start' ? ' active' : ''}"`,
      `href="${page.route}" data-section="${page.id}" class="nav-link${active}"${current}`,
    )
    result = result.replaceAll(`href="#${page.id}"`, `href="${page.route}"`)
  }
  return result
}

function articleSection(page) {
  let section = sections.get(page.id)
  if (page.id !== 'start') {
    section = section.replace('<h2>', '<h1 itemprop="headline">').replace('</h2>', '</h1>')
  } else {
    section = section.replace('<h1 class="sr-only">', '<h1 class="sr-only" itemprop="headline">')
  }
  return routeNavigation(section, page.id)
}

function breadcrumbs(page) {
  if (page.id === 'start') return ''
  return `<nav class="breadcrumbs" aria-label="面包屑"><a href="/">文档</a><span>/</span><span>${escapeHTML(page.group)}</span><span>/</span><span aria-current="page">${escapeHTML(page.title)}</span></nav>`
}

function render(page) {
  const url = `${origin}${page.route}`
  const documentTitle = page.id === 'start' && page.title === 'AnyToken API 文档'
    ? page.title
    : `${page.title} | AnyToken 文档`
  const navigation = routeNavigation(sidebar, page.id)
  const pageHeader = routeNavigation(header, page.id)
  const article = articleSection(page)
  const pageFooter = routeNavigation(footer, page.id)
  const pageOverlays = absoluteAssets(overlays)
  return `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="description" content="${escapeHTML(page.description)}" />
    <meta name="robots" content="index,follow,max-image-preview:large" />
    <meta name="theme-color" content="#0b0d0c" />
    <link rel="canonical" href="${url}" />
    <meta property="og:type" content="${page.id === 'start' ? 'website' : 'article'}" />
    <meta property="og:site_name" content="AnyToken 文档" />
    <meta property="og:title" content="${escapeHTML(documentTitle)}" />
    <meta property="og:description" content="${escapeHTML(page.description)}" />
    <meta property="og:url" content="${url}" />
    <meta name="twitter:card" content="summary" />
    <title>${escapeHTML(documentTitle)}</title>
    <link rel="icon" href="/assets/logo.svg" />
    <link rel="stylesheet" href="/styles.css" />
  </head>
  <body data-page="${page.id}">
    <a class="skip-link" href="#main">跳到正文</a>
    ${absoluteAssets(pageHeader)}
    <div class="shell">
      ${navigation}
      <main id="main" class="content">
        <article itemscope itemtype="https://schema.org/TechArticle">
          <meta itemprop="name" content="${escapeHTML(page.title)}" />
          <meta itemprop="description" content="${escapeHTML(page.description)}" />
          <link itemprop="url" href="${url}" />
          ${breadcrumbs(page)}
          ${article}
          ${pageFooter}
        </article>
        <aside class="toc" aria-label="本页目录">
          <p>本页内容</p>
          <div class="toc-links"></div>
          <a class="back-top" href="#${page.id}">回到顶部 ↑</a>
        </aside>
      </main>
    </div>
    ${pageOverlays.replace('<script src="/app.js" defer></script>', '<script src="/search-index.js" defer></script>\n    <script src="/app.js" defer></script>')}
  </body>
</html>
`
}

await rm(output, { recursive: true, force: true })
await mkdir(output, { recursive: true })
await cp(path.join(root, 'assets'), path.join(output, 'assets'), { recursive: true })
await cp(path.join(root, 'styles.css'), path.join(output, 'styles.css'))
await cp(path.join(root, 'app.js'), path.join(output, 'app.js'))

for (const page of pages) {
  const directory = page.route === '/' ? output : path.join(output, page.route)
  await mkdir(directory, { recursive: true })
  await writeFile(path.join(directory, 'index.html'), render(page))
}

const searchIndex = pages.map((page) => {
  const plainText = sections.get(page.id)
    .replace(/<script[\s\S]*?<\/script>/gi, ' ')
    .replace(/<style[\s\S]*?<\/style>/gi, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
  return { id: page.id, url: page.route, title: page.title, description: page.description, text: `${page.title} ${page.description} ${plainText}`.toLowerCase() }
})

await writeFile(path.join(output, 'search-index.json'), `${JSON.stringify(searchIndex, null, 2)}\n`)
await writeFile(path.join(output, 'search-index.js'), `window.__ANYTOKEN_SEARCH_INDEX__ = ${JSON.stringify(searchIndex)};\n`)
await writeFile(path.join(output, 'robots.txt'), `User-agent: *\nAllow: /\n\nSitemap: ${origin}/sitemap.xml\n`)
await writeFile(path.join(output, 'sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${pages.map((page) => `  <url><loc>${origin}${page.route}</loc></url>`).join('\n')}
</urlset>\n`)

const notFound = render({ ...pages[0], id: 'start', title: '页面未找到', description: '你访问的 AnyToken 文档页面不存在。' })
  .replace(articleSection(pages[0]), '<section class="doc-section"><div class="section-no">404 / NOT FOUND</div><h1>页面未找到</h1><p>该文档地址不存在或已经调整。</p><a class="button primary" href="/">返回文档首页 →</a></section>')
  .replace('<meta name="robots" content="index,follow,max-image-preview:large" />', '<meta name="robots" content="noindex,follow" />')
await writeFile(path.join(output, '404.html'), notFound)

console.log(`Built ${pages.length} static documentation pages in ${output}`)
