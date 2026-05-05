# 量化终端 Docker 部署指南

## 前置要求

- Docker 20.10+
- Docker Compose v2+

## 快速部署

### 1. 克隆项目

```bash
git clone <repo-url>
cd <repo-dir>
```

### 2. 启动服务

```bash
docker compose up -d
```

首次启动会自动构建镜像，约需 2-5 分钟。

### 3. 访问系统

浏览器打开 `http://<服务器IP>`

### 4. 配置 Token

首次使用需要在"设置"页面配置 Tushare API Token。

## 迁移已有数据库

如果已有 `stocks.db` 数据库文件，可以在启动前放入 Docker volume：

```bash
# 先创建 volume
docker volume create github_stock-data

# 将数据库复制到 volume 中
docker run --rm \
  -v github_stock-data:/data \
  -v /path/to/stocks.db:/backup/stocks.db \
  alpine cp /backup/stocks.db /data/stocks.db

# 然后启动服务
docker compose up -d
```

> 注意：volume 名称格式为 `<项目目录名>_stock-data`。可以用 `docker volume ls` 查看实际名称。

## 常用命令

| 操作 | 命令 |
|------|------|
| 查看日志 | `docker compose logs -f` |
| 查看后端日志 | `docker compose logs -f backend` |
| 停止服务 | `docker compose down` |
| 重新构建 | `docker compose build --no-cache` |
| 更新代码后部署 | `docker compose up -d --build` |
| 查看运行状态 | `docker compose ps` |

## 数据备份

数据库存储在 Docker volume 中，备份命令：

```bash
# 备份
docker run --rm \
  -v github_stock-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/stock-data-backup.tar.gz -C /data .

# 恢复
docker run --rm \
  -v github_stock-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/stock-data-backup.tar.gz -C /data
```

## 自定义端口

编辑 `docker-compose.yml`，修改 frontend 的 ports 映射：

```yaml
frontend:
  ports:
    - "8080:80"  # 改为通过 8080 端口访问
```

然后重启：`docker compose up -d`

## 开发模式

如果不使用 Docker，可以直接运行：

```bash
# 后端
cd stock-backend
go run .

# 前端（新终端）
cd stock-frontend
npm install
npm run dev
```

开发模式下前端通过 Vite 代理连接后端 `localhost:8081`。

## 架构说明

```
浏览器 → :80 (nginx)
           ├── /         → 前端静态文件
           └── /api/*    → 反向代理 → 后端:8081
```

- **nginx**: 服务前端 + 反向代理 API 请求
- **后端**: Go API 服务，端口 8081（仅内部访问）
- **数据库**: SQLite 文件，通过 Docker volume 持久化
