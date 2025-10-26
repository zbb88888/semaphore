# RunnerProxy 配置示例

## 基本用法

```go
package main

import (
    "time"
    "github.com/semaphoreui/semaphore/services/runner_proxy"
)

func main() {
    // 配置 RunnerProxy
    config := &runnerproxy.ProxyConfig{
        // 外部 manager API (提供 releases 数据)
        ManagerURL: "http://manager-api:8080",

        // Semaphore API (用于创建 templates 和 tasks)
        SemaphoreURL: "http://semaphore:3000",

        NodeName:          "deploy-node-001",
        RunnerID:          "runner-001",
        BinProxyVersion:   "1.0.0",
        KeepAliveInterval: 30 * time.Second,
    }

    // 创建 RunnerProxy 实例
    proxy := runnerproxy.NewRunnerProxy(config)

    // 启动服务 (自动获取 releases 并创建 runners)
    go proxy.Run()

    // 示例：为特定 bin 启动部署
    if err := proxy.StartDeploymentForBin("my-app"); err != nil {
        log.Printf("Deployment failed: %v", err)
    }

    // 查看状态
    status, _ := proxy.GetRunnerStatus("my-app")
    log.Printf("Runner status: %+v", status)
}
```

## API 映射

### 外部 Manager API (ManagerURL)

- `GET /api/v1/releases` - 获取发布列表
- `POST /api/v1/keepalive` - 节点注册
- `GET /api/v1/keepalive?node_id=<id>` - 查询节点状态

### Semaphore API (SemaphoreURL)

- `POST /api/v1/projects/{id}/templates` - 创建模板
- `POST /api/v1/projects/{id}/templates/{id}/tasks` - 创建任务
- `POST /api/v1/projects/{id}/tasks/{id}/confirm` - 确认任务
- `GET /api/v1/projects/{id}/tasks/{id}` - 获取任务状态
- `GET /api/v1/projects/{id}/tasks/{id}/output` - 获取任务输出

## 数据流

1. **获取 Releases**: `ManagerURL/api/v1/releases` → `RunnerProxy.ListReleases()`
2. **解析 BinsToDo**: `releases` → `map[string]string` (bin → version)
3. **创建 Runner**: `bin` → `Runner{templates, tasks}`
4. **创建 Templates**: `SemaphoreURL/api/v1/projects/{id}/templates`
5. **创建 Tasks**: `SemaphoreURL/api/v1/projects/{id}/templates/{id}/tasks`
6. **执行部署**: 按顺序执行四个阶段的任务

## 重要说明

✅ **正确做法**: RunnerProxy 作为 HTTP 客户端调用现有的 Semaphore API
❌ **错误做法**: 修改 Semaphore 的 router.go 或其他核心文件

RunnerProxy 完全独立于 Semaphore 核心代码，通过标准的 HTTP API 进行交互。
