# RunnerProxy Bin to Runner Implementation

本实现为 RunnerProxy 添加了将 bin 对应到 runner 的功能，支持在 runner 中创建 template 和 task。

## 架构概述

```
外部 Manager API --> RunnerProxy (HTTP Client) --> Semaphore API
```

- **外部 Manager API**: 提供 releases 接口 (`ManagerURL`)
- **RunnerProxy**: HTTP 客户端，负责 bin 到 runner 的映射
- **Semaphore API**: 现有的成熟 API，用于创建 templates 和 tasks (`SemaphoreURL`)

**重要**: RunnerProxy **不修改** Semaphore 的 router.go，而是通过 HTTP 客户端调用现有的 API。

## 配置说明

```go
type ProxyConfig struct {
    ManagerURL      string // 外部 manager API URL (用于获取 releases)
    SemaphoreURL    string // Semaphore API URL (用于创建 templates 和 tasks)
    NodeName        string
    RunnerID        string
    BinProxyVersion string
    HTTPClient      *http.Client
}
```

### 1. Bin 到 Runner 的映射

RunnerProxy 会自动从 manager 获取 releases 列表，并为每个需要部署的 bin 创建对应的 Runner：

```go
// 在 fetchAndLogReleases() 中实现
for bin, version := range binsToDo {
    err := rp.createOrUpdateRunner(bin, version, ReleasesToDo)
    // ...
}
```

### 2. Runner 结构

每个 Runner 维护四个部署阶段的模板和任务：

```go
type Runner struct {
    projectID int
    AppName   string
    GitLink   string

    // 四个部署阶段的模板 ID
    preDownloadTemplateID int
    deployTemplateID      int
    postCheckTemplateID   int
    rollBackTemplateID    int

    // 四个部署阶段的任务 ID
    preDownloadTaskID int
    deployTaskID      int
    postCheckTaskID   int
    rollBackTaskID    int

    // 部署参数
    Environment  string
    Strategy     string
    Version      string
    DownloadLink string
}
```

### 3. Template 创建

为每个 Runner 自动创建四个 Ansible playbook 模板：

- `pre_download.yml` - 预下载阶段
- `deploy.yml` - 部署阶段
- `post_check.yml` - 部署后检查
- `roll_back.yml` - 回滚阶段

### 4. Task 创建和执行

基于 template 创建对应的 task，支持：

- 按顺序执行部署任务
- 如果 post_check 失败，自动执行回滚
- 手动启动回滚操作
- 查询任务状态和输出

## 核心方法

### RunnerProxy 方法

```go
// 为指定 bin 启动部署流程
func (rp *RunnerProxy) StartDeploymentForBin(binName string) error

// 为指定 bin 启动回滚操作
func (rp *RunnerProxy) StartRollbackForBin(binName string) error

// 获取指定 bin 的 runner 状态
func (rp *RunnerProxy) GetRunnerStatus(binName string) (map[string]interface{}, error)

// 获取所有 runners 的状态
func (rp *RunnerProxy) GetAllRunnerStatuses() (map[string]interface{}, error)
```

### Runner 方法

```go
// 启动完整部署流程（四个阶段按顺序执行）
func (r *Runner) StartDeployment(proxy *RunnerProxy) error

// 手动启动回滚操作
func (r *Runner) StartRollback(proxy *RunnerProxy) error

// 获取 Runner 状态
func (r *Runner) GetStatus(proxy *RunnerProxy) (map[string]interface{}, error)
```

## 部署流程

1. **Pre-download**: 下载应用包和依赖
2. **Deploy**: 执行实际部署操作
3. **Post-check**: 验证部署是否成功
4. **Roll-back**: 如果检查失败或手动触发，执行回滚

## 使用示例

```go
// 创建 RunnerProxy
config := &runnerproxy.ProxyConfig{
    ManagerURL: "http://localhost:3000",
    NodeName:   "test-node",
    RunnerID:   "runner-001",
}
proxy := runnerproxy.NewRunnerProxy(config)

// 启动服务（自动获取 releases 并创建 runners）
go proxy.Run()

// 为特定 bin 启动部署
err := proxy.StartDeploymentForBin("my-app")

// 查看状态
status, err := proxy.GetRunnerStatus("my-app")

// 如果需要回滚
err := proxy.StartRollbackForBin("my-app")
```

## 配置说明

- `ManagerURL`: Semaphore manager 的 URL
- `NodeName`: 节点名称
- `RunnerID`: Runner ID，用于向 manager 注册
- `KeepAliveInterval`: 心跳间隔时间

## 文件结构

- `proxy.go`: 主要的 RunnerProxy 实现，包含 bin 到 runner 映射逻辑
- `runner.go`: Runner 结构和部署流程控制
- `template.go`: Template 相关的 HTTP 客户端方法
- `task.go`: Task 相关的 HTTP 客户端方法

## 特性

- ✅ 自动从 manager 获取 releases
- ✅ 为每个 bin 创建对应的 Runner
- ✅ 自动创建四阶段部署模板和任务
- ✅ 支持按顺序执行部署流程
- ✅ 支持自动回滚（post-check 失败时）
- ✅ 支持手动回滚
- ✅ 支持状态查询和任务输出获取
- ✅ 错误处理和超时控制
