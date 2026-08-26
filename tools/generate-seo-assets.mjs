import { readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const checkOnly = process.argv.includes('--check')

const main = JSON.parse(await readFile(path.join(root, 'seo/main-pages.json'), 'utf8'))
const docs = JSON.parse(await readFile(path.join(root, 'docs-site/pages.json'), 'utf8'))

function validateSite(site, label) {
  const origin = new URL(site.origin)
  if (origin.pathname !== '/' || origin.search || origin.hash) {
    throw new Error(`${label} origin must not include a path, query or fragment`)
  }
  const routes = new Set()
  for (const page of site.pages) {
    if (typeof page.route !== 'string' || !page.route.startsWith('/') || page.route.includes('?') || page.route.includes('#')) {
      throw new Error(`${label} has invalid route: ${page.route}`)
    }
    if (routes.has(page.route)) throw new Error(`${label} has duplicate route: ${page.route}`)
    if (!/^\d{4}-\d{2}-\d{2}$/.test(page.lastmod || '')) throw new Error(`${label} has invalid lastmod: ${page.route}`)
    routes.add(page.route)
  }
}

function absoluteURL(origin, route) {
  return route === '/' ? `${origin}/` : `${origin}${route}`
}

function sitemap(site) {
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${site.pages.map((page) => `  <url><loc>${absoluteURL(site.origin, page.route)}</loc><lastmod>${page.lastmod}</lastmod></url>`).join('\n')}
</urlset>
`
}

const outputs = new Map([
  [path.join(root, 'frontend/public/robots.txt'), `User-agent: *\nAllow: /\n\nSitemap: ${main.origin}/sitemap.xml\n`],
  [path.join(root, 'frontend/public/sitemap.xml'), sitemap(main)],
  [path.join(root, 'urls.txt'), `${[...main.pages.map((page) => absoluteURL(main.origin, page.route)), ...docs.pages.map((page) => absoluteURL(docs.origin, page.route))].join('\n')}\n`],
])

let stale = false
for (const [file, expected] of outputs) {
  if (checkOnly) {
    const actual = await readFile(file, 'utf8').catch(() => '')
    if (actual !== expected) {
      stale = true
      console.error(`SEO asset is stale: ${path.relative(root, file)}`)
    }
  } else {
    await writeFile(file, expected)
    console.log(`Generated ${path.relative(root, file)}`)
  }
}

if (stale) process.exitCode = 1
