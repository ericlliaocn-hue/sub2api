import { cp, mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.dirname(fileURLToPath(import.meta.url))
const output = path.join(root, 'dist')
const checkOnly = process.argv.includes('--check')
const catalog = JSON.parse(await readFile(path.join(root, 'pages.json'), 'utf8'))
const origin = catalog.origin
const pages = catalog.pages

if (new Set(pages.map((page) => page.id)).size !== pages.length) throw new Error('Duplicate documentation page id')
if (new Set(pages.map((page) => page.route)).size !== pages.length) throw new Error('Duplicate documentation route')
if (new Set(pages.map((page) => page.title)).size !== pages.length) throw new Error('Duplicate documentation page title')
if (new Set(pages.map((page) => page.description)).size !== pages.length) throw new Error('Duplicate documentation page description')

for (const page of pages) {
  if (page.route !== '/' && !/^\/[a-z0-9/-]+\/$/.test(page.route)) throw new Error(`Invalid documentation route: ${page.route}`)
  if (!/^\d{4}-\d{2}-\d{2}$/.test(page.lastmod)) throw new Error(`Invalid lastmod: ${page.route}`)
  if (!page.title.trim() || !page.description.trim()) throw new Error(`Missing metadata: ${page.route}`)
}

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
  return `<nav class="breadcrumbs" aria-label="面包屑" itemscope itemtype="https://schema.org/BreadcrumbList"><span itemprop="itemListElement" itemscope itemtype="https://schema.org/ListItem"><a href="/" itemprop="item"><span itemprop="name">文档</span></a><meta itemprop="position" content="1" /></span><span>/</span><span itemprop="itemListElement" itemscope itemtype="https://schema.org/ListItem"><span itemprop="name">${escapeHTML(page.group)}</span><meta itemprop="position" content="2" /></span><span>/</span><span itemprop="itemListElement" itemscope itemtype="https://schema.org/ListItem"><span aria-current="page" itemprop="name">${escapeHTML(page.title)}</span><meta itemprop="position" content="3" /></span></nav>`
}

function relatedNavigation(page) {
  const index = pages.findIndex((candidate) => candidate.id === page.id)
  const previous = index > 0 ? pages[index - 1] : null
  const next = index < pages.length - 1 ? pages[index + 1] : null
  const links = [
    previous ? `<a rel="prev" href="${previous.route}"><span>上一篇</span><strong>${escapeHTML(previous.title)}</strong></a>` : '',
    next ? `<a rel="next" href="${next.route}"><span>下一篇</span><strong>${escapeHTML(next.title)}</strong></a>` : '',
  ].filter(Boolean).join('')
  return links ? `<nav class="related-pages" aria-label="相关文档">${links}</nav>` : ''
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
          <meta itemprop="dateModified" content="${escapeHTML(page.lastmod)}" />
          <link itemprop="url" href="${url}" />
          ${breadcrumbs(page)}
          ${article}
          ${relatedNavigation(page)}
          ${pageFooter}
        </article>
        <aside class="toc" aria-label="本页目录">
          <p>本页内容</p>
          <div class="toc-links"></div>
          <a class="back-top" href="#${page.id}">回到顶部 ↑</a>
        </aside>
      </main>
    </div>
    ${pageOverlays.replace('<script src="/app.js" defer></script>', '<script src="/search-index.js" defer></script>\n    <script src="/app.js" defer></script>\n    <script charset="UTF-8" id="LA_COLLECT" src="//sdk.51.la/js-sdk-pro.min.js" defer></script>\n    <script src="/analytics.js" defer></script>')}
  </body>
</html>
`
}

function occurrences(html, pattern) {
  return [...html.matchAll(pattern)].length
}

function validateRenderedPage(page, html) {
  const expectedCanonical = `${origin}${page.route}`
  if (occurrences(html, /<h1(?:\s|>)/g) !== 1) throw new Error(`Expected exactly one H1: ${page.route}`)
  if (occurrences(html, /<title>/g) !== 1) throw new Error(`Expected exactly one title: ${page.route}`)
  if (occurrences(html, /<meta name="description"/g) !== 1) throw new Error(`Expected exactly one description: ${page.route}`)
  if (occurrences(html, /<link rel="canonical"/g) !== 1 || !html.includes(`href="${expectedCanonical}"`)) {
    throw new Error(`Invalid canonical: ${page.route}`)
  }
  if (!html.includes('itemtype="https://schema.org/TechArticle"')) throw new Error(`Missing TechArticle schema: ${page.route}`)
  if (page.id !== 'start' && !html.includes('itemtype="https://schema.org/BreadcrumbList"')) {
    throw new Error(`Missing breadcrumb schema: ${page.route}`)
  }
}

let stale = false

async function emit(file, content) {
  if (checkOnly) {
    const current = await readFile(file, 'utf8').catch(() => '')
    if (current !== content) {
      stale = true
      console.error(`Documentation output is stale: ${path.relative(root, file)}`)
    }
    return
  }
  await mkdir(path.dirname(file), { recursive: true })
  await writeFile(file, content)
}

if (!checkOnly) {
  await rm(output, { recursive: true, force: true })
  await mkdir(output, { recursive: true })
  await cp(path.join(root, 'assets'), path.join(output, 'assets'), { recursive: true })
  await cp(path.join(root, 'styles.css'), path.join(output, 'styles.css'))
  await cp(path.join(root, 'app.js'), path.join(output, 'app.js'))
  await cp(path.join(root, 'analytics.js'), path.join(output, 'analytics.js'))
} else {
  await emit(path.join(output, 'styles.css'), await readFile(path.join(root, 'styles.css'), 'utf8'))
  await emit(path.join(output, 'app.js'), await readFile(path.join(root, 'app.js'), 'utf8'))
  await emit(path.join(output, 'analytics.js'), await readFile(path.join(root, 'analytics.js'), 'utf8'))
}

for (const page of pages) {
  const directory = page.route === '/' ? output : path.join(output, page.route)
  const html = render(page)
  validateRenderedPage(page, html)
  await emit(path.join(directory, 'index.html'), html)
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

await emit(path.join(output, 'search-index.json'), `${JSON.stringify(searchIndex, null, 2)}\n`)
await emit(path.join(output, 'search-index.js'), `window.__ANYTOKEN_SEARCH_INDEX__ = ${JSON.stringify(searchIndex)};\n`)
await emit(path.join(output, 'robots.txt'), `User-agent: *\nAllow: /\n\nSitemap: ${origin}/sitemap.xml\n`)
await emit(path.join(output, 'sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${pages.map((page) => `  <url><loc>${origin}${page.route}</loc><lastmod>${page.lastmod}</lastmod></url>`).join('\n')}
</urlset>\n`)

const notFound = render({ ...pages[0], id: 'start', route: '/404.html', title: '页面未找到', description: '你访问的 AnyToken 文档页面不存在。' })
  .replace(articleSection(pages[0]), '<section class="doc-section"><div class="section-no">404 / NOT FOUND</div><h1>页面未找到</h1><p>该文档地址不存在或已经调整。</p><a class="button primary" href="/">返回文档首页 →</a></section>')
  .replace('<meta name="robots" content="index,follow,max-image-preview:large" />', '<meta name="robots" content="noindex,follow" />')
  .replace(/\n    <link rel="canonical"[^>]+>/, '')
  .replace(/\n    <meta property="og:url"[^>]+>/, '')
await emit(path.join(output, '404.html'), notFound)

if (stale) {
  process.exitCode = 1
} else {
  console.log(`${checkOnly ? 'Verified' : 'Built'} ${pages.length} static documentation pages in ${output}`)
}
