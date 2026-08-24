# AnyToken 文档站

这是 `doc.anytoken.work` 的独立多页面静态文档站，不依赖第三方 CDN、Google Fonts 或 JavaScript 运行时框架。构建后每篇文章都有独立 URL 和完整 HTML，便于搜索引擎抓取。

## 本地预览

先生成静态站：

```bash
cd docs-site
npm run build
npm run preview
```

然后访问 `http://127.0.0.1:4174`。

## 发布

将 `docs-site/dist/` 中的全部文件同步到文档站静态目录，并让 `doc.anytoken.work` 的 Nginx 虚拟主机指向该目录。发布前需确认：

1. `doc.anytoken.work` 已解析到生产服务器；
2. TLS 证书覆盖 `doc.anytoken.work`；
3. Nginx 对 `index.html`、字体、CSS、JavaScript 和 SVG 返回正确 MIME 类型；
4. 生产环境不为文档域名开放模型 API 或管理 API 路由。
5. Nginx 使用 `try_files $uri $uri/ =404;`，使 `/api/responses/` 等目录型静态地址正确返回文章页面。
6. Nginx 配置 `error_page 404 /404.html;`，并将 `/404.html` 设为仅内部错误页，确保不存在的地址返回设计好的静态 404 页面和真实 HTTP 404 状态。

构建会自动生成：

- 每篇文章独立的 `index.html`；
- 每页独立的 title、description、canonical 与 Open Graph 元数据；
- `robots.txt`、`sitemap.xml`、`search-index.json` 和兼容严格 CSP 的 `search-index.js`；
- 不参与索引的 `404.html`。

## 内容口径

- 官网与控制台：`https://anytoken.work`
- 模型 API：`https://api.anytoken.work`
- OpenAI 兼容 Base URL：`https://api.anytoken.work/v1`
- 文档：`https://doc.anytoken.work`

客户端配置应与 `frontend/src/components/keys/UseKeyModal.vue` 保持一致。模型名和价格是动态数据，文档只引导用户到模型广场查询，不维护静态价格表。
