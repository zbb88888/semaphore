package runnerproxy

// RunnerProxy is an HTTP client for communicating with manager API server
// It provides methods to interact with the following manager APIs:
// GET /api/v1/keepalive?node_id=<id> - 查询节点状态
// POST /api/v1/keepalive - 注册节点
// List /api/v1/releases - 获取发布列表
// 请求体: {"node_id": "string", "cpu_arch": "string", "os_release": "string", "node_name": "string", "bin_proxy_version": "string"}
// GET /api/v1/bins/:bin_name - 获取二进制文件信息
// POST /api/v1/bins/:bin_name - 更新节点的二进制文件版本
// 请求体: {"node_id": "string", "sha256sum": "string"}
// POST /api/v1/bins/:bin_name/progress - 记录二进制文件更新进度
// 请求体: {"nodeName": "string", "targetHash": "string", "status": "string", "processingTime": int}
// GET /api/v1/download/:bin_file_name - 下载二进制文件
// GET /health - 健康检查

// RunnerProxy 只是一个 HTTP 客户端，不运行后台服务，需要时才与 manager 交互

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/semaphoreui/semaphore/db"
)

// ProxyConfig holds configuration for the runner proxy
type ProxyConfig struct {
	// NeedRegister      bool

	ManagerURL      string // 外部 manager API URL
	SemaphoreURL    string // Semaphore API URL (用于创建 templates 和 tasks)
	ManagerIsActive bool
	NodeName        string
	RunnerID        string
	BinProxyVersion string
	HTTPClient      *http.Client

	KeepAliveInterval time.Duration
	// 待部署的服务列表：包含 bin 服务类型
	ReleasesToDo []Release

	// 把 release 里的 bin 名字和 version 提取出来
	BinsToDo map[string]string

	// 每一个 bin 都对应一个 Runner: 对应到 semaphore template，然后基于 template 创建 semaphore task，最后 把 version 变量传给 task 并启动
	Runners map[string]*Runner
}

// RunnerProxy manages communication with the manager
type RunnerProxy struct {
	config *ProxyConfig
}

// NewRunnerProxy creates a new runner proxy instance
func NewRunnerProxy(config *ProxyConfig) *RunnerProxy {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if config.KeepAliveInterval == 0 {
		config.KeepAliveInterval = 30 * time.Second
	}
	return &RunnerProxy{config: config}
}

// KeepAliveRequest represents the request body for node registration
type KeepAliveRequest struct {
	CPUArch         string `json:"cpu_arch"`
	OSRelease       string `json:"os_release"`
	NodeName        string `json:"node_name"`
	NodeID          string `json:"node_id"` // 这个应该为 runner id，每个集群就是一个 runner
	BinProxyVersion string `json:"bin_proxy_version"`
}

// BinaryUpdateRequest represents the request body for binary version updates
type BinaryUpdateRequest struct {
	NodeName  string `json:"node_id"`
	SHA256Sum string `json:"sha256sum"`
}

// ProgressRequest represents the request body for progress updates
type ProgressRequest struct {
	NodeName       string `json:"nodeName"`
	TargetHash     string `json:"targetHash"`
	Status         string `json:"status"`
	ProcessingTime int    `json:"processingTime"`
}

// CheckNodeStatus queries node status via GET /api/v1/keepalive
func (rp *RunnerProxy) CheckNodeStatus() error {
	url := fmt.Sprintf("%s/api/v1/keepalive?node_id=%s", rp.config.ManagerURL, rp.config.RunnerID)
	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to check node status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("node status check failed with status: %d", resp.StatusCode)
	}
	return nil
}

// RegisterNode registers the node via POST /api/v1/keepalive
func (rp *RunnerProxy) RegisterNode() error {
	NodeID := rp.config.RunnerID
	request := KeepAliveRequest{
		NodeID:          NodeID,
		NodeName:        rp.config.NodeName,
		CPUArch:         runtime.GOARCH,
		OSRelease:       runtime.GOOS,
		BinProxyVersion: rp.config.BinProxyVersion,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/keepalive", rp.config.ManagerURL)
	resp, err := rp.config.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("node registration failed with status: %d", resp.StatusCode)
	}

	return nil
}

type Release struct {
	ID            string     `json:"id" bson:"_id,omitempty"`
	ProjectID     string     `json:"projectId" bson:"projectId"`
	ProjectName   string     `json:"projectName" bson:"projectName"`
	ApplicationID string     `json:"applicationId" bson:"applicationId"`
	Version       string     `json:"version" bson:"version"`
	Environment   string     `json:"environment" bson:"environment"`
	Strategy      string     `json:"strategy" bson:"strategy"`
	Status        string     `json:"status" bson:"status"`
	Description   string     `json:"description" bson:"description"`
	Scheduler     string     `json:"scheduler" bson:"scheduler"`
	GitlabPRURL   string     `json:"gitlabPrUrl" bson:"gitlabPrUrl"`
	TarFileName   string     `json:"tarFileName" bson:"tarFileName"`
	StartedAt     *time.Time `json:"startedAt,omitempty" bson:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt" bson:"createdAt"`
}

type ReleaseResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// GetBinaryInfo retrieves binary file information via GET /api/v1/bins/:bin_name
func (rp *RunnerProxy) GetBinaryInfo(binName string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/bins/%s", rp.config.ManagerURL, binName)
	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get binary info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get binary info failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// UpdateBinaryVersion updates node's binary file version via POST /api/v1/bins/:bin_name
func (rp *RunnerProxy) UpdateBinaryVersion(binName, sha256sum string) error {
	request := BinaryUpdateRequest{
		NodeName:  rp.config.NodeName,
		SHA256Sum: sha256sum,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/bins/%s", rp.config.ManagerURL, binName)
	resp, err := rp.config.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to update binary version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binary version update failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ReportProgress reports binary file update progress via POST /api/v1/bins/:bin_name/progress
func (rp *RunnerProxy) ReportProgress(binName, targetHash, status string, processingTime int) error {
	request := ProgressRequest{
		NodeName:       rp.config.NodeName,
		TargetHash:     targetHash,
		Status:         status,
		ProcessingTime: processingTime,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/bins/%s/progress", rp.config.ManagerURL, binName)
	resp, err := rp.config.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to report progress: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("progress report failed with status: %d", resp.StatusCode)
	}

	return nil
}

// DownloadBinary downloads binary file via GET /api/v1/download/:bin_file_name
func (rp *RunnerProxy) DownloadBinary(binFileName string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/download/%s", rp.config.ManagerURL, binFileName)
	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binary download failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read binary data: %w", err)
	}

	return body, nil
}

// ListReleases retrieves release list via GET /api/v1/releases
func (rp *RunnerProxy) ListReleases() (*ReleaseResponse, error) {
	url := fmt.Sprintf("%s/api/v1/releases", rp.config.ManagerURL)
	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list releases failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var releaseResp ReleaseResponse
	if err := json.Unmarshal(body, &releaseResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal release response (body: %s): %w", string(body), err)
	}

	return &releaseResp, nil
}

// HealthCheck performs health check via GET /health
func (rp *RunnerProxy) HealthCheck() error {
	url := fmt.Sprintf("%s/health", rp.config.ManagerURL)
	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to perform health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rp.config.ManagerIsActive = false
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	rp.config.ManagerIsActive = true
	return nil
}

// NewProxyService creates a new runner proxy service with default configuration
// This is a convenience function for simple setups
// The RunnerProxy is just an HTTP client to interact with manager API server
func NewProxyService() *RunnerProxy {
	// 获取 linux 操作系统的名字
	nodeName, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %w", err))
	}
	config := &ProxyConfig{
		ManagerURL:        "http://localhost:38012", // Default manager URL
		SemaphoreURL:      "http://localhost:3000",  // Default Semaphore URL
		NodeName:          nodeName,                 // Auto-detected node name
		BinProxyVersion:   "1.0.0",                  // Default version
		KeepAliveInterval: 10 * time.Second,         // Default interval
		HTTPClient:        &http.Client{Timeout: 30 * time.Second},
		RunnerID:          "manage-gen-runner-id-001",
	}
	return NewRunnerProxy(config)
}

// 实现一个 run 函数
func (rp *RunnerProxy) Run() {
	fmt.Printf("Runner Proxy started. Manager URL: %s, Node: %s\n", rp.config.ManagerURL, rp.config.NodeName)

	// 创建两个定时器：一个用于 keepalive，一个用于拉取 releases
	keepaliveTicker := time.NewTicker(rp.config.KeepAliveInterval)
	releasesTicker := time.NewTicker(1 * time.Minute) // 每分钟拉取一次 releases
	defer keepaliveTicker.Stop()
	defer releasesTicker.Stop()

	// 启动时立即拉取一次 releases
	rp.fetchAndLogReleases()

	for {
		select {
		case <-keepaliveTicker.C:
			// 每隔 KeepAliveInterval 调用 HealthCheck 方法
			err := rp.HealthCheck()
			if err != nil {
				fmt.Printf("Health check failed: %v\n", err)
			}
			if !rp.config.ManagerIsActive {
				fmt.Println("Manager is not active. Skipping further checks.")
				continue
			}
			// 调用 CheckNodeStatus 方法
			err = rp.CheckNodeStatus()
			if err != nil {
				fmt.Printf("Node status check failed: %v\n", err)
				// 如果节点状态检查失败，则调用 RegisterNode 方法注册节点
				err = rp.RegisterNode()
				if err != nil {
					fmt.Printf("Node registration failed: %v\n", err)
				}
			}
		case <-releasesTicker.C:
			// 定期拉取 releases 列表
			rp.fetchAndLogReleases()
		}
	}
}

// fetchAndLogReleases 拉取并记录 releases 信息
func (rp *RunnerProxy) fetchAndLogReleases() {
	if !rp.config.ManagerIsActive {
		fmt.Println("Manager is not active. Skipping releases fetch.")
		return
	}

	releases, err := rp.ListReleases()
	if err != nil {
		fmt.Printf("Failed to fetch releases: %v\n", err)
		return
	}

	// 解析 Data 字段为 Release 数组
	if releases.Data == nil {
		fmt.Println("No releases data returned from manager")
		return
	}

	// 尝试将 Data 转换为 Release 数组
	dataBytes, err := json.Marshal(releases.Data)
	if err != nil {
		fmt.Printf("Failed to marshal releases data: %v\n", err)
		return
	}

	var releaseList []Release
	if err := json.Unmarshal(dataBytes, &releaseList); err != nil {
		fmt.Printf("Failed to unmarshal releases data: %v\n", err)
		return
	}

	fmt.Printf("Fetched %d releases from manager:\n", len(releaseList))
	ReleasesToDo := []Release{}
	for _, release := range releaseList {
		if release.Status == "completed" && release.TarFileName != "" {
			// applicationId version environment strategy tarFileName
			fmt.Printf("  - Project: %s, App: %s, Version: %s, Environment: %s, Strategy: %s, TarFile: %s\n",
				release.ProjectName, release.ApplicationID, release.Version,
				release.Environment, release.Strategy, release.TarFileName)
			ReleasesToDo = append(ReleasesToDo, release)
		}
	}
	//TODO:// diff to refresh ReleasesToDo
	rp.config.ReleasesToDo = ReleasesToDo
	fmt.Printf("%d Releases to do updated.\n", len(ReleasesToDo))
	binsToDo := make(map[string]string)
	// 把 release 里的 bin 名字和 version 提取出来
	for _, release := range ReleasesToDo {
		binsToDo[release.ApplicationID] = release.Version
	}
	rp.config.BinsToDo = binsToDo
	// 调用 runner.template.Search 函数 找到 bin 对应的 semaphore 的 ansible-playbook 任务模板
	for bin, version := range binsToDo {
		// 为每个 bin 创建或更新对应的 Runner
		err := rp.createOrUpdateRunner(bin, version, ReleasesToDo)
		if err != nil {
			fmt.Printf("Failed to create/update runner for bin %s: %v\n", bin, err)
			continue
		}
		fmt.Printf("Successfully created/updated runner for bin %s version %s\n", bin, version)
	}

}

// createOrUpdateRunner 为指定的 bin 创建或更新 Runner
func (rp *RunnerProxy) createOrUpdateRunner(bin, version string, releases []Release) error {
	// 从 releases 中找到对应的 release 信息
	var targetRelease *Release
	for _, release := range releases {
		if release.ApplicationID == bin {
			targetRelease = &release
			break
		}
	}

	if targetRelease == nil {
		return fmt.Errorf("no release found for bin: %s", bin)
	}

	// 初始化 Runners map 如果为空
	if rp.config.Runners == nil {
		rp.config.Runners = make(map[string]*Runner)
	}

	// 检查是否已存在该 bin 的 runner
	runner, exists := rp.config.Runners[bin]
	if !exists {
		// 创建新的 runner
		runner = &Runner{
			AppName:     bin,
			Environment: targetRelease.Environment,
			Strategy:    targetRelease.Strategy,
			Version:     version,
			GitLink:     targetRelease.GitlabPRURL, // 使用 GitlabPRURL 作为 GitLink
		}
		rp.config.Runners[bin] = runner
		fmt.Printf("Created new runner for bin: %s\n", bin)
	} else {
		// 更新现有 runner 的版本信息
		runner.Version = version
		runner.Environment = targetRelease.Environment
		runner.Strategy = targetRelease.Strategy
		fmt.Printf("Updated existing runner for bin: %s\n", bin)
	}

	// 为 runner 创建 templates 和 tasks
	err := rp.setupRunnerTemplatesAndTasks(runner, targetRelease)
	if err != nil {
		return fmt.Errorf("failed to setup templates and tasks for runner %s: %w", bin, err)
	}

	return nil
}

// setupRunnerTemplatesAndTasks 为 runner 设置 templates 和 tasks
func (rp *RunnerProxy) setupRunnerTemplatesAndTasks(runner *Runner, release *Release) error {
	// 解析 projectID
	projectID := 1 // 默认项目ID，实际应该从配置或 release 中获取
	if release.ProjectID != "" {
		// 这里可以添加字符串到整数的转换逻辑
		// 为简化，暂时使用默认值
	}

	runner.projectID = projectID

	// 创建四个部署阶段的 templates
	err := rp.createDeploymentTemplates(runner, projectID)
	if err != nil {
		return fmt.Errorf("failed to create deployment templates: %w", err)
	}

	// 基于 templates 创建对应的 tasks
	err = rp.createDeploymentTasks(runner, projectID)
	if err != nil {
		return fmt.Errorf("failed to create deployment tasks: %w", err)
	}

	return nil
}

// createDeploymentTemplates 创建四个部署阶段的 templates
func (rp *RunnerProxy) createDeploymentTemplates(runner *Runner, projectID int) error {
	// 创建 pre_download template
	preDownloadTemplate, err := rp.createTemplate(projectID, runner, "pre_download")
	if err != nil {
		return fmt.Errorf("failed to create pre_download template: %w", err)
	}
	runner.preDownloadTemplateID = preDownloadTemplate.ID

	// 创建 deploy template
	deployTemplate, err := rp.createTemplate(projectID, runner, "deploy")
	if err != nil {
		return fmt.Errorf("failed to create deploy template: %w", err)
	}
	runner.deployTemplateID = deployTemplate.ID

	// 创建 post_check template
	postCheckTemplate, err := rp.createTemplate(projectID, runner, "post_check")
	if err != nil {
		return fmt.Errorf("failed to create post_check template: %w", err)
	}
	runner.postCheckTemplateID = postCheckTemplate.ID

	// 创建 roll_back template
	rollBackTemplate, err := rp.createTemplate(projectID, runner, "roll_back")
	if err != nil {
		return fmt.Errorf("failed to create roll_back template: %w", err)
	}
	runner.rollBackTemplateID = rollBackTemplate.ID

	fmt.Printf("Created all deployment templates for runner %s\n", runner.AppName)
	return nil
}

// createTemplate 创建指定阶段的 template
func (rp *RunnerProxy) createTemplate(projectID int, runner *Runner, stage string) (*db.Template, error) {
	// 需要导入 db 包
	template := db.Template{
		Name:         fmt.Sprintf("%s-%s-%s", runner.AppName, stage, runner.Environment),
		ProjectID:    projectID,
		Playbook:     fmt.Sprintf("deploy/%s.yml", stage),
		Arguments:    &[]string{fmt.Sprintf("--extra-vars version=%s environment=%s strategy=%s", runner.Version, runner.Environment, runner.Strategy)}[0],
		Description:  &[]string{fmt.Sprintf("Deployment template for %s %s stage", runner.AppName, stage)}[0],
		App:          db.AppAnsible,
		RepositoryID: 1, // 默认仓库ID，实际应该从配置中获取
		// 设置其他必要的字段
		TaskParams: map[string]interface{}{
			"version":     runner.Version,
			"environment": runner.Environment,
			"strategy":    runner.Strategy,
			"app_name":    runner.AppName,
		},
	}

	createdTemplate, err := rp.CreateTemplate(projectID, template)
	if err != nil {
		return nil, fmt.Errorf("failed to create template for stage %s: %w", stage, err)
	}

	fmt.Printf("Created template %s (ID: %d) for runner %s\n", createdTemplate.Name, createdTemplate.ID, runner.AppName)
	return createdTemplate, nil
}

// createDeploymentTasks 创建四个部署阶段的 tasks
func (rp *RunnerProxy) createDeploymentTasks(runner *Runner, projectID int) error {
	// 创建 pre_download task
	preDownloadTask, err := rp.createTask(projectID, runner.preDownloadTemplateID, runner, "pre_download")
	if err != nil {
		return fmt.Errorf("failed to create pre_download task: %w", err)
	}
	runner.preDownloadTaskID = preDownloadTask.ID

	// 创建 deploy task
	deployTask, err := rp.createTask(projectID, runner.deployTemplateID, runner, "deploy")
	if err != nil {
		return fmt.Errorf("failed to create deploy task: %w", err)
	}
	runner.deployTaskID = deployTask.ID

	// 创建 post_check task
	postCheckTask, err := rp.createTask(projectID, runner.postCheckTemplateID, runner, "post_check")
	if err != nil {
		return fmt.Errorf("failed to create post_check task: %w", err)
	}
	runner.postCheckTaskID = postCheckTask.ID

	// 创建 roll_back task
	rollBackTask, err := rp.createTask(projectID, runner.rollBackTemplateID, runner, "roll_back")
	if err != nil {
		return fmt.Errorf("failed to create roll_back task: %w", err)
	}
	runner.rollBackTaskID = rollBackTask.ID

	fmt.Printf("Created all deployment tasks for runner %s\n", runner.AppName)
	return nil
}

// createTask 创建指定阶段的 task
func (rp *RunnerProxy) createTask(projectID, templateID int, runner *Runner, stage string) (*db.Task, error) {
	task := db.Task{
		ProjectID:   projectID,
		TemplateID:  templateID,
		Playbook:    fmt.Sprintf("deploy/%s.yml", stage),
		Arguments:   &[]string{fmt.Sprintf("--extra-vars version=%s environment=%s strategy=%s app_name=%s", runner.Version, runner.Environment, runner.Strategy, runner.AppName)}[0],
		Environment: fmt.Sprintf("%s-%s", runner.AppName, runner.Environment),
		Message:     fmt.Sprintf("Deploy %s %s in %s environment using %s strategy", runner.AppName, runner.Version, runner.Environment, runner.Strategy),
		Params: map[string]interface{}{
			"version":     runner.Version,
			"environment": runner.Environment,
			"strategy":    runner.Strategy,
			"app_name":    runner.AppName,
			"stage":       stage,
		},
	}

	createdTask, err := rp.CreateTask(projectID, templateID, task)
	if err != nil {
		return nil, fmt.Errorf("failed to create task for stage %s: %w", stage, err)
	}

	fmt.Printf("Created task %s (ID: %d) for runner %s\n", stage, createdTask.ID, runner.AppName)
	return createdTask, nil
}

// StartDeploymentForBin 为指定的 bin 启动部署流程
func (rp *RunnerProxy) StartDeploymentForBin(binName string) error {
	if rp.config.Runners == nil {
		return fmt.Errorf("no runners configured")
	}

	runner, exists := rp.config.Runners[binName]
	if !exists {
		return fmt.Errorf("no runner found for bin: %s", binName)
	}

	return runner.StartDeployment(rp)
}

// StartRollbackForBin 为指定的 bin 启动回滚操作
func (rp *RunnerProxy) StartRollbackForBin(binName string) error {
	if rp.config.Runners == nil {
		return fmt.Errorf("no runners configured")
	}

	runner, exists := rp.config.Runners[binName]
	if !exists {
		return fmt.Errorf("no runner found for bin: %s", binName)
	}

	return runner.StartRollback(rp)
}

// GetRunnerStatus 获取指定 bin 的 runner 状态
func (rp *RunnerProxy) GetRunnerStatus(binName string) (map[string]interface{}, error) {
	if rp.config.Runners == nil {
		return nil, fmt.Errorf("no runners configured")
	}

	runner, exists := rp.config.Runners[binName]
	if !exists {
		return nil, fmt.Errorf("no runner found for bin: %s", binName)
	}

	return runner.GetStatus(rp)
}

// GetAllRunnerStatuses 获取所有 runners 的状态
func (rp *RunnerProxy) GetAllRunnerStatuses() (map[string]interface{}, error) {
	if rp.config.Runners == nil {
		return map[string]interface{}{}, nil
	}

	statuses := make(map[string]interface{})
	for binName, runner := range rp.config.Runners {
		status, err := runner.GetStatus(rp)
		if err != nil {
			statuses[binName] = map[string]interface{}{
				"error": err.Error(),
			}
		} else {
			statuses[binName] = status
		}
	}

	return statuses, nil
}
