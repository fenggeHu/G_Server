# Database Specification

本项目的数据库表和字段命名规范。

## 表命名规则

1. **表名使用单数形式。** 如玩家表使用 `player`，不使用 `players`。
2. 表名使用 snake_case 小写，单词之间用下划线分隔。

## 通用字段

### 必选字段

所有表默认包含以下 2 个字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `created_at` | `timestamptz not null default now()` | 该行记录的创建时间，默认数据库的系统时间，创建时生成，不能被修改 |
| `updated_at` | `timestamptz not null default now()` | 该行记录的更新时间，每次修改该行时更新为数据库的当前时间 |

### 可选字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | `smallint not null default 1` | 该行记录的状态（可选） |

## 表结构清单

| 表名 | created_at | updated_at | status | 备注 |
|------|:---:|:---:|:---:|------|
| `player` | Y | Y | Y | 玩家表 |
| `session` | Y | Y | Y | 会话表 |
| `ticket` | Y | Y | Y | 票据表 |
| `room` | Y | Y | - | 房间表（有业务级 `room_status` 字段） |
| `room_reservation` | Y | Y | Y | 房间预约表 |
| `player_progress` | Y | Y | Y | 玩家进度表 |
| `active_player_session` | Y | Y | Y | 活跃玩家会话表 |
| `progress_operation` | Y | Y | - | 进度操作表（有业务级 `status` 字段） |
| `schema_migration` | Y | Y | Y | 迁移记录表 |

## 变更记录

| 日期 | 变更 |
|------|------|
| 2026-09-10 | 初始规范：确立单数表名、必选 created_at/updated_at、可选 status 字段 |