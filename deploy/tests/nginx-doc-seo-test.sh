#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
nginx_bin="${NGINX_BIN:-$(command -v nginx)}"
mime_types="${NGINX_MIME_TYPES:-/opt/homebrew/etc/nginx/mime.types}"
docs_root="${repo_root}/docs-site/dist"
http_port="$((20000 + RANDOM % 10000))"
site_port="$((30000 + RANDOM % 10000))"
test_root="$(mktemp -d /tmp/anytoken-nginx-doc-seo.XXXXXX)"

case "${test_root}" in
  /tmp/anytoken-nginx-doc-seo.*) ;;
  *) echo "Unsafe temporary directory: ${test_root}" >&2; exit 1 ;;
esac

cleanup() {
  "${nginx_bin}" -p "${test_root}/" -c "${test_root}/nginx.conf" -s stop >/dev/null 2>&1 || true
  rm -rf "${test_root}"
}
trap cleanup EXIT

test -f "${docs_root}/index.html"
mkdir -p "${test_root}/conf.d" "${test_root}/logs"
cp "${repo_root}/deploy/nginx/conf.d/doc.anytoken.work.conf" "${test_root}/conf.d/doc.anytoken.work.conf"

sed -i.bak -E \
  -e "s/listen 80;/listen ${http_port};/" \
  -e "s/listen 443 ssl;/listen ${site_port};/" \
  -e "s#root /home/git/nginx/html/doc\.anytoken\.work;#root ${docs_root};#" \
  -e 's#^[[:space:]]*access_log .*#    access_log off;#' \
  -e 's#^[[:space:]]*error_log .*#    error_log stderr;#' \
  -e '/^[[:space:]]*ssl_/d' \
  "${test_root}/conf.d/doc.anytoken.work.conf"

cp "${repo_root}/deploy/tests/fixtures/nginx-doc-seo.conf" "${test_root}/nginx.conf"
sed -i.bak \
  -e "s#__TEST_ROOT__#${test_root}#g" \
  -e "s#__MIME_TYPES__#${mime_types}#g" \
  "${test_root}/nginx.conf"

"${nginx_bin}" -t -p "${test_root}/" -c "${test_root}/nginx.conf"
"${nginx_bin}" -p "${test_root}/" -c "${test_root}/nginx.conf"

curl_args=(--noproxy '*' --max-time 5 --silent --show-error -H 'Host: doc.anytoken.work')
curl "${curl_args[@]}" --dump-header "${test_root}/page.headers" --output "${test_root}/page.html" "http://127.0.0.1:${site_port}/quickstart/"
curl "${curl_args[@]}" --dump-header "${test_root}/missing.headers" --output "${test_root}/missing.html" "http://127.0.0.1:${site_port}/not-found"
curl "${curl_args[@]}" --dump-header "${test_root}/robots.headers" --output "${test_root}/robots.txt" "http://127.0.0.1:${site_port}/robots.txt"
curl "${curl_args[@]}" --dump-header "${test_root}/css.headers" --output /dev/null "http://127.0.0.1:${site_port}/styles.css"

grep -Eq '^HTTP/[^ ]+ 200' "${test_root}/page.headers"
grep -Eqi '^Content-Type: text/html' "${test_root}/page.headers"
grep -Eqi '^Cache-Control: no-cache' "${test_root}/page.headers"
grep -Fq '<link rel="canonical" href="https://doc.anytoken.work/quickstart/"' "${test_root}/page.html"
grep -Fq '<meta name="robots" content="index,follow,max-image-preview:large"' "${test_root}/page.html"
grep -Eq '^HTTP/[^ ]+ 404' "${test_root}/missing.headers"
grep -Fq '<meta name="robots" content="noindex,follow"' "${test_root}/missing.html"
! grep -Fq '<link rel="canonical"' "${test_root}/missing.html"
grep -Eqi '^Content-Type: text/plain' "${test_root}/robots.headers"
grep -Fq 'Sitemap: https://doc.anytoken.work/sitemap.xml' "${test_root}/robots.txt"
grep -Eqi '^Cache-Control: max-age=3600' "${test_root}/css.headers"

echo "Nginx documentation SEO checks passed"
