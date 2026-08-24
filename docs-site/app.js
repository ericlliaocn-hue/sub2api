const sections = [...document.querySelectorAll('.doc-section')]
const navLinks = [...document.querySelectorAll('.nav-link')]
const toast = document.querySelector('.toast')
const searchModal = document.querySelector('.search-modal')
const searchInput = document.querySelector('.search-input')
const searchResults = document.querySelector('.search-results')
const menuButton = document.querySelector('.mobile-menu')

const localSearchableSections = sections.map((section) => ({
  id: section.id,
  url: `#${section.id}`,
  title: section.querySelector('h1, h2')?.textContent?.trim() || section.id,
  description: section.querySelector('.lead, p')?.textContent?.trim() || '',
  text: `${section.dataset.search || ''} ${section.textContent || ''}`.toLowerCase(),
}))
const searchableSections = Array.isArray(window.__ANYTOKEN_SEARCH_INDEX__) && window.__ANYTOKEN_SEARCH_INDEX__.length
  ? window.__ANYTOKEN_SEARCH_INDEX__
  : localSearchableSections

const toc = document.querySelector('.toc-links')
sections.forEach((section) => {
  const heading = section.querySelector('h1, h2')
  if (!heading) return
  const link = document.createElement('a')
  link.href = `#${section.id}`
  link.textContent = heading.textContent
  link.dataset.section = section.id
  toc.append(link)
})

function setActiveSection(id) {
  navLinks.forEach((link) => {
    const matches = link.dataset.section === id || link.hash === `#${id}`
    link.classList.toggle('active', matches)
  })
  document.querySelectorAll('.toc-links a').forEach((link) => {
    link.classList.toggle('active', link.dataset.section === id)
  })
}

const sectionObserver = new IntersectionObserver((entries) => {
  const visible = entries
    .filter((entry) => entry.isIntersecting)
    .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
  if (visible[0]) setActiveSection(visible[0].target.id)
}, { rootMargin: '-12% 0px -72% 0px', threshold: 0 })

sections.forEach((section) => sectionObserver.observe(section))

function showToast(message = '已复制') {
  toast.textContent = message
  toast.classList.add('show')
  window.clearTimeout(showToast.timer)
  showToast.timer = window.setTimeout(() => toast.classList.remove('show'), 1400)
}

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    showToast()
  } catch {
    showToast('复制失败，请手动复制')
  }
}

document.querySelectorAll('[data-copy]').forEach((button) => {
  button.addEventListener('click', () => copyText(button.dataset.copy))
})

document.querySelectorAll('.copy-code').forEach((button) => {
  button.addEventListener('click', () => {
    const code = button.closest('.code-block')?.querySelector('code')?.textContent || ''
    copyText(code.trim())
  })
})

document.querySelectorAll('[data-tabs]').forEach((tabs) => {
  tabs.querySelectorAll('.tab').forEach((tab) => {
    tab.addEventListener('click', () => {
      tabs.querySelectorAll('.tab').forEach((item) => item.classList.toggle('active', item === tab))
      tabs.querySelectorAll('.tab-panel').forEach((panel) => {
        panel.classList.toggle('active', panel.dataset.panel === tab.dataset.tab)
      })
    })
  })
})

function renderSearch(query = '') {
  const normalized = query.trim().toLowerCase()
  const matches = normalized
    ? searchableSections.filter((item) => item.text.includes(normalized)).slice(0, 12)
    : searchableSections.slice(0, 8)

  searchResults.replaceChildren()
  if (!matches.length) {
    const empty = document.createElement('div')
    empty.className = 'search-empty'
    empty.textContent = '没有找到相关内容'
    searchResults.append(empty)
    return
  }

  matches.forEach((item) => {
    const link = document.createElement('a')
    link.className = 'search-result'
    link.href = item.url || `#${item.id}`
    const title = document.createElement('strong')
    title.textContent = item.title
    const description = document.createElement('span')
    description.textContent = item.description.slice(0, 76)
    link.append(title, description)
    link.addEventListener('click', closeSearch)
    searchResults.append(link)
  })
}

function openSearch() {
  searchModal.hidden = false
  document.body.style.overflow = 'hidden'
  renderSearch(searchInput.value)
  window.setTimeout(() => searchInput.focus(), 0)
}

function closeSearch() {
  searchModal.hidden = true
  document.body.style.overflow = ''
}

document.querySelector('.search-trigger').addEventListener('click', openSearch)
document.querySelector('.search-backdrop').addEventListener('click', closeSearch)
searchInput.addEventListener('input', () => renderSearch(searchInput.value))

document.addEventListener('keydown', (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    openSearch()
  }
  if (event.key === 'Escape' && !searchModal.hidden) closeSearch()
})

menuButton.addEventListener('click', () => {
  const isOpen = document.body.classList.toggle('menu-open')
  menuButton.setAttribute('aria-expanded', String(isOpen))
})

navLinks.forEach((link) => {
  link.addEventListener('click', () => {
    document.body.classList.remove('menu-open')
    menuButton.setAttribute('aria-expanded', 'false')
  })
})
