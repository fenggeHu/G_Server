# AI Agent 入口（服务端仓库）

本仓库含 `Backend/`（Go module）与 `GameServer/`（Godot headless）。开始前：

1. 读集合规范：
   - `../docs/AI_Agent三方并行开发规范.md`
   - 绝对路径：`/Users/max/aigame/docs/`
   - 规范不可达时停止写代码并向用户报告。

2. 角色来自任务卡；本仓库默认归 Server Agent。按任务卡授权路径工作。

3. 默认不得修改 `3D_App` 仓库。不得加载客户端 FBX/材质/UI，也不得接受任意资源路径。

4. 测试命令（各自 workdir）：
   - `Backend/`：`go test -count=1 ./...`
   - `GameServer/`：`godot --headless --path . --script tests/<test>.gd`

5. 数据库 schema/迁移、RPC 语义、房间与票据状态由 Server owner 唯一修改，且需先走契约变更。

6. 提交前：`git status --short`；只暂存本任务文件；报告提交号、测试结果与未测内容。
