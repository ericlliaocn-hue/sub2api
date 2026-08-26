# 非 Docker 本地开发环境

本地开发使用 Homebrew PostgreSQL 16、Redis、Go 后端和 Vite 前端，不依赖 Docker。

## 首次配置

```bash
cp backend/.env.local.example backend/.env.local
chmod 600 backend/.env.local
```

编辑 `backend/.env.local`，至少填写：

- `DATABASE_PASSWORD`：本机 PostgreSQL 的 `sub2api` 用户密码。
- `ADMIN_PASSWORD`：首次初始化空数据库时创建的管理员密码，至少 8 个字符。
- `JWT_SECRET`、`TOTP_ENCRYPTION_KEY`：分别执行 `openssl rand -hex 32` 生成。

本机需提供：

```bash
brew install go postgresql@16 redis
brew services start postgresql@16
brew services start redis
corepack prepare pnpm@9.15.9 --activate
```

脚本会在检测到其他 pnpm 主版本时，通过 Corepack 自动激活 pnpm 9.15.9，并使用官方 npm registry 安装依赖，不修改锁文件。项目的 `go.mod` 会通过 Go 自动工具链使用锁定版本；脚本在 Apple Silicon Mac 上优先使用 `/opt/homebrew/opt/go/bin/go`，避免误用 Rosetta 下的 amd64 Go。

## 运行

```bash
./tools/local-dev.sh start
```

启动成功后：

- 前端：http://127.0.0.1:3000
- 后端：http://127.0.0.1:8080
- 健康检查：http://127.0.0.1:8080/health
- 日志：`.dev/local/backend.log`、`.dev/local/frontend.log`

首次启动空数据库时，后端会自动执行迁移，并使用 `ADMIN_EMAIL` / `ADMIN_PASSWORD` 创建管理员。生成的 `backend/config.yaml`、`backend/.installed` 和本地环境文件均已被 Git 忽略。

## 常用命令

```bash
./tools/local-dev.sh doctor     # 检查工具链和数据服务连接
./tools/local-dev.sh status     # 查看状态
./tools/local-dev.sh logs       # 跟踪前后端日志
./tools/local-dev.sh restart    # 重新构建并重启
./tools/local-dev.sh stop       # 仅停止前后端
```

`stop` 不会停止 PostgreSQL/Redis，避免影响本机其他项目。如需停止 Homebrew 服务，可手动执行：

```bash
brew services stop postgresql@16
brew services stop redis
```
