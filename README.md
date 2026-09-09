# 游戏服务端

状态：当前服务端工程，Go Backend 与 Godot Headless GameServer 均为正式实现入口。

## 职责

根目录 `Server/` 使用 Go 和领域服务设计，承载用户身份认证、会话、入场票据、健康检查、玩家进度、配置 API 及房间控制面；Godot Headless GameServer 承载实时房间、权威运动物理、AI 与战斗。数据库使用 PostgreSQL。`../3D_App/` 承载 Godot 手机客户端，Xcode 负责 iOS 构建、签名、调试与发布。

首版 Go 采用模块化单体，不因领域拆分立即部署微服务；GS 独立作为房间运行时。identity、session、progress、configuration 和 room control 按实际用例逐步实现，不创建空服务。Go 领域层维护权限/事务规则，GS 判定玩法事实；Go 不解析客户端与 GS 的 Godot 原生 RPC。接口归使用方所有。

## 工程规则

- Go Backend 负责身份、session、票据、进度、配置 API 和房间控制面；GameServer 负责实时玩法。客户端与 GS 使用锁定版本的 Godot 原生 RPC；Go 不解析该协议，只校验控制面权限和幂等事务。
- 实时传输必须单独选型并验证 Godot/Go 互通。WSS 可作为候选，但尚未批准，不承诺不可靠独立通道。
- GS 执行 Godot CharacterBody3D/碰撞、权威运动、AI 和战斗；同引擎仍需版本、资源和运行环境校正，角色运动尚未实现，不承诺共享。
- 客户端 Shared 是协议源；GameServer 构建时复制并检查 hash，稳定后可外提共享 package。

权威决策：[根目录与技术栈决策](../docs/根目录与技术栈决策.md)。
