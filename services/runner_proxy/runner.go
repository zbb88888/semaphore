package runnerproxy

import (
	"fmt"
	"time"
)

// 1. runner 负责维护与 manager 的交互的数据结构以及方法
// 2. runner 需要定期向 manager 发送心跳请求，以保持连接
// 3. runner 负责把 GET /api/v1/bins/:bin_name 的部署任务转变为 semaphore 的任务模板，对应 projects.AddTemplate 函数
// 4. 创建完 template 之后，需要基于 template 创建一个 task 并启动，对应 projects.ConfirmTask 函数

// Runner represents a runner proxy that communicates with the manager
// Runner 维护了版本发布本身的信息以及和Semaphore template 和 task 的对应关系
// manager 给的发布信息数据： projectid applicationId version environment strategy
type Runner struct {
	projectID int
	AppName   string // 对应 appidlicationId

	// 每个 app 都有唯一的仓库地址，默认部署模板就在仓库的 deploy 目录下
	GitLink string // 对应 github 仓库 Link

	// ansible-playbook 模板
	// 部署过程分为四个阶段：包含四个 semaphore template
	preDownloadTemplateID int // 对应文件名： githubURL/deploy/pre_download.yml
	deployTemplateID      int // 对应文件名： githubURL/deploy/deploy.yml
	postCheckTemplateID   int // 对应文件名： githubURL/deploy/post_check.yml
	rollBackTemplateID    int // 对应文件名： githubURL/deploy/roll_back.yml

	// 每个模板都只对应一个 semaphore task
	preDownloadTaskID int
	deployTaskID      int
	postCheckTaskID   int
	rollBackTaskID    int

	// ansible 环境变量
	Environment  string // 对应 environment
	Strategy     string // 对应 strategy
	Version      string // 对应 version
	DownloadLink string // 对应 downloadLink
}

// StartDeployment 启动部署流程，按顺序执行四个阶段的任务
func (r *Runner) StartDeployment(proxy *RunnerProxy) error {
	fmt.Printf("Starting deployment for %s version %s in %s environment\n", r.AppName, r.Version, r.Environment)

	// 阶段 1: Pre-download
	if err := r.startAndWaitForTask(proxy, r.preDownloadTaskID, "pre_download"); err != nil {
		return fmt.Errorf("pre_download stage failed: %w", err)
	}

	// 阶段 2: Deploy
	if err := r.startAndWaitForTask(proxy, r.deployTaskID, "deploy"); err != nil {
		return fmt.Errorf("deploy stage failed: %w", err)
	}

	// 阶段 3: Post-check
	if err := r.startAndWaitForTask(proxy, r.postCheckTaskID, "post_check"); err != nil {
		fmt.Errorf("post_check stage failed: %w", err)
		// 如果 post-check 失败，执行回滚
		fmt.Printf("Post-check failed, initiating rollback for %s\n", r.AppName)
		if rollbackErr := r.startAndWaitForTask(proxy, r.rollBackTaskID, "roll_back"); rollbackErr != nil {
			return fmt.Errorf("rollback also failed: %w (original error: %w)", rollbackErr, err)
		}
		return err
	}

	fmt.Printf("Deployment completed successfully for %s version %s\n", r.AppName, r.Version)
	return nil
}

// startAndWaitForTask 启动指定的任务并等待完成
func (r *Runner) startAndWaitForTask(proxy *RunnerProxy, taskID int, stageName string) error {
	fmt.Printf("Starting %s stage (task ID: %d) for %s\n", stageName, taskID, r.AppName)

	// 确认任务启动
	err := proxy.ConfirmTask(r.projectID, taskID)
	if err != nil {
		return fmt.Errorf("failed to confirm %s task: %w", stageName, err)
	}

	// 等待任务完成
	err = r.waitForTaskCompletion(proxy, taskID, stageName)
	if err != nil {
		return fmt.Errorf("%s task execution failed: %w", stageName, err)
	}

	fmt.Printf("Completed %s stage for %s\n", stageName, r.AppName)
	return nil
}

// waitForTaskCompletion 等待任务完成
func (r *Runner) waitForTaskCompletion(proxy *RunnerProxy, taskID int, stageName string) error {
	maxWaitTime := 30 * time.Minute   // 最大等待时间
	checkInterval := 10 * time.Second // 检查间隔

	startTime := time.Now()
	for {
		// 检查是否超时
		if time.Since(startTime) > maxWaitTime {
			return fmt.Errorf("task %s (ID: %d) timed out after %v", stageName, taskID, maxWaitTime)
		}

		// 获取任务状态
		task, err := proxy.GetTask(r.projectID, taskID)
		if err != nil {
			fmt.Printf("Error checking task status: %v, retrying in %v\n", err, checkInterval)
			time.Sleep(checkInterval)
			continue
		}

		// 检查任务状态
		switch task.Status {
		case "success":
			return nil
		case "error", "stopped":
			// 获取任务输出以了解失败原因
			output, outputErr := proxy.GetTaskOutput(r.projectID, taskID)
			if outputErr != nil {
				return fmt.Errorf("task failed with status: %s (unable to get output: %v)", task.Status, outputErr)
			}
			return fmt.Errorf("task failed with status: %s, output: %s", task.Status, string(output))
		case "running", "pending":
			fmt.Printf("Task %s is %s, waiting...\n", stageName, task.Status)
			time.Sleep(checkInterval)
		default:
			fmt.Printf("Task %s has unknown status: %s, continuing to wait...\n", stageName, task.Status)
			time.Sleep(checkInterval)
		}
	}
}

// StartRollback 手动启动回滚操作
func (r *Runner) StartRollback(proxy *RunnerProxy) error {
	fmt.Printf("Starting rollback for %s in %s environment\n", r.AppName, r.Environment)

	err := r.startAndWaitForTask(proxy, r.rollBackTaskID, "roll_back")
	if err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	fmt.Printf("Rollback completed successfully for %s\n", r.AppName)
	return nil
}

// GetStatus 获取 Runner 的当前状态
func (r *Runner) GetStatus(proxy *RunnerProxy) (map[string]interface{}, error) {
	status := map[string]interface{}{
		"app_name":    r.AppName,
		"version":     r.Version,
		"environment": r.Environment,
		"strategy":    r.Strategy,
		"project_id":  r.projectID,
		"templates": map[string]int{
			"pre_download": r.preDownloadTemplateID,
			"deploy":       r.deployTemplateID,
			"post_check":   r.postCheckTemplateID,
			"roll_back":    r.rollBackTemplateID,
		},
		"tasks": map[string]int{
			"pre_download": r.preDownloadTaskID,
			"deploy":       r.deployTaskID,
			"post_check":   r.postCheckTaskID,
			"roll_back":    r.rollBackTaskID,
		},
	}

	// 获取各个任务的状态
	taskStatuses := make(map[string]string)

	// 检查 pre_download 任务状态
	if task, err := proxy.GetTask(r.projectID, r.preDownloadTaskID); err == nil {
		taskStatuses["pre_download"] = string(task.Status)
	}

	// 检查 deploy 任务状态
	if task, err := proxy.GetTask(r.projectID, r.deployTaskID); err == nil {
		taskStatuses["deploy"] = string(task.Status)
	}

	// 检查 post_check 任务状态
	if task, err := proxy.GetTask(r.projectID, r.postCheckTaskID); err == nil {
		taskStatuses["post_check"] = string(task.Status)
	}

	// 检查 roll_back 任务状态
	if task, err := proxy.GetTask(r.projectID, r.rollBackTaskID); err == nil {
		taskStatuses["roll_back"] = string(task.Status)
	}

	status["task_statuses"] = taskStatuses

	return status, nil
}
