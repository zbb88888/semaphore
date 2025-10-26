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
)

// ProxyConfig holds configuration for the runner proxy
type ProxyConfig struct {
	// NeedRegister      bool

	ManagerURL      string
	ManagerIsActive bool
	NodeName        string
	RunnerID        string
	BinProxyVersion string
	HTTPClient      *http.Client

	KeepAliveInterval time.Duration
	// 待部署的服务列表：包含 bin 服务类型
	ReleasesToDo []Release
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
	// 一旦有了新的 ReleasesToDo，就去获取对应的二进制文件信息
	for _, release := range ReleasesToDo {
		binInfo, err := rp.GetBinaryInfo(release.ApplicationID)
		if err != nil {
			fmt.Printf("Failed to get binary info for %s: %v\n", release.ApplicationID, err)
			continue
		}
		fmt.Printf("Binary info for %s: %s\n", release.ApplicationID, string(binInfo))
	}
}
