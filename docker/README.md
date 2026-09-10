# 本地 Docker 开发环境

启动 Go Backend、Godot GameServer 和 PostgreSQL：

```sh
cd Server/docker
`.env` 是已提交的本地开发配置模板，只包含开发占位凭据；可按本机修改。真实环境使用未跟踪的 `.env.local`，或通过 shell 环境变量覆盖。
docker compose -f compose.yaml up --build
```

服务地址：

- Backend API：`http://10.10.13.140:18080`（宿主机局域网地址）
- GameServer ENet：`127.0.0.1:17000/udp`（宿主机和容器监听端口一致）
- PostgreSQL：`127.0.0.1:55432`

`ROOM_PRESET` defaults to `exploration`; set it to `coop_combat` in `.env` before starting a combat test. The client must use the same preset.

这是开发配置，使用明文 HTTP 和 ENet，只适用于受控局域网，不得用于公网。Backend 绑定 `0.0.0.0` 以允许 iPhone 访问；GameServer 注册 `ROOM_HOST=10.10.13.140`，必须是客户端可达的宿主机局域网地址。修改 `.env` 中的 `BACKEND_BIND_IP`、`ROOM_HOST` 以匹配实际网卡。

Backend 启动时执行 migration 和开发账号 seed。`G0_DEV_PASSWORD` 通过环境变量传入，不写入镜像或日志。不要将 `.env.local`、生产凭据或证书提交。开发卷删除：

```sh
docker compose -f compose.yaml down -v
```

GameServer 镜像默认从 Godot 官方 GitHub release 下载精确版本；可在构建时传入 `GODOT_SHA256` 做完整性校验。若镜像源或架构不匹配，先在 CI 固定并缓存该基础工具链。

Dockerfile 根据 BuildKit 的 `TARGETARCH` 自动选择 Godot 官方 Linux `arm64` 或 `x86_64` 包。Apple Silicon 本机使用原生 `linux/arm64`，不会运行 AMD64 仿真；Intel/AMD 主机使用 `linux/amd64`。Backend 同样由 Go 基础镜像自动选择本机架构。Backend 启动时会自动执行 migration 和开发账号 seed。
