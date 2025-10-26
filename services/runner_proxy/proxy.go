package runnerproxy

// RunnerProxy is an HTTP client for communicating with manager API server
// It provides methods to interact with the following manager APIs:
// GET /api/v1/keepalive?node_id=<id> - 查询节点状态
// POST /api/v1/keepalive - 注册节点
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
	"runtime"
	"time"
)

// ProxyConfig holds configuration for the runner proxy
type ProxyConfig struct {
	ManagerURL        string
	NodeID            string
	NodeName          string
	BinProxyVersion   string
	KeepAliveInterval time.Duration
	HTTPClient        *http.Client
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
	NodeID          string `json:"node_id"`
	CPUArch         string `json:"cpu_arch"`
	OSRelease       string `json:"os_release"`
	NodeName        string `json:"node_name"`
	BinProxyVersion string `json:"bin_proxy_version"`
}

// BinaryUpdateRequest represents the request body for binary version updates
type BinaryUpdateRequest struct {
	NodeID    string `json:"node_id"`
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
	url := fmt.Sprintf("%s/api/v1/keepalive?node_id=%s", rp.config.ManagerURL, rp.config.NodeID)
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
	request := KeepAliveRequest{
		NodeID:          rp.config.NodeID,
		CPUArch:         runtime.GOARCH,
		OSRelease:       runtime.GOOS,
		NodeName:        rp.config.NodeName,
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
		NodeID:    rp.config.NodeID,
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

// HealthCheck performs health check via GET /health
func (rp *RunnerProxy) HealthCheck() error {
	url := fmt.Sprintf("%s/health", rp.config.ManagerURL)
	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to perform health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// NewProxyService creates a new runner proxy service with default configuration
// This is a convenience function for simple setups
// The RunnerProxy is just an HTTP client to interact with manager API server
func NewProxyService() *RunnerProxy {
	config := &ProxyConfig{
		ManagerURL:        "http://localhost:3000", // Default manager URL
		NodeID:            "default-node",          // Default node ID
		NodeName:          "semaphore-node",        // Default node name
		BinProxyVersion:   "1.0.0",                 // Default version
		KeepAliveInterval: 30 * time.Second,        // Default interval
		HTTPClient:        &http.Client{Timeout: 30 * time.Second},
	}
	return NewRunnerProxy(config)
}
