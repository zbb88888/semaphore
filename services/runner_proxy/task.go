package runnerproxy

// 1. runner.go 的 Runner 对象会调用 这里的 Task 对象，启动 bin 部署任务
// 2. 一旦任务开始，Task 对象会 跟踪 Task 进度，会与 manager 进行交互，更新任务状态
