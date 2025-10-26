package runnerproxy

// 1. runner 负责维护与 manager 的交互的数据结构以及方法
// 2. runner 需要定期向 manager 发送心跳请求，以保持连接
// 3. runner 负责把 GET /api/v1/bins/:bin_name 的部署任务转变为 semaphore 的任务模板，对应 projects.AddTemplate 函数
// 4. 创建完 template 之后，需要基于 template 创建一个 task 并启动，对应 projects.ConfirmTask 函数
