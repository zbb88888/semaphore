package runnerproxy

// 1. runner 负责维护与 manager 的交互的数据结构以及方法
// 2. runner 需要定期向 manager 发送心跳请求，以保持连接
// 3. runner 负责把 GET /api/v1/bins/:bin_name 的部署任务转变为 semaphore 的任务模板，对应 projects.AddTemplate 函数
// 4. 创建完 template 之后，需要基于 template 创建一个 task 并启动，对应 projects.ConfirmTask 函数

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/semaphoreui/semaphore/db"
	"github.com/semaphoreui/semaphore/pkg/task_logger"
)

// Runner represents a runner proxy that communicates with the manager
type Runner struct {
	ID           string
	ManagerURL   string
	HeartbeatURL string
	BinURL       string
	client       *http.Client
	store        db.Store
	projectID    int
}

// NewRunner creates a new runner instance
func NewRunner(id, managerURL string, projectID int, store db.Store) *Runner {
	return &Runner{
		ID:           id,
		ManagerURL:   managerURL,
		HeartbeatURL: fmt.Sprintf("%s/api/v1/runners/%s/heartbeat", managerURL, id),
		BinURL:       fmt.Sprintf("%s/api/v1/bins", managerURL),
		client:       &http.Client{Timeout: 30 * time.Second},
		store:        store,
		projectID:    projectID,
	}
}

// HeartbeatPayload represents the heartbeat request payload
type HeartbeatPayload struct {
	RunnerID  string    `json:"runner_id"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// BinDeployment represents a deployment task from manager
type BinDeployment struct {
	BinName     string            `json:"bin_name"`
	Command     string            `json:"command"`
	Environment map[string]string `json:"environment"`
	WorkingDir  string            `json:"working_dir"`
}

// SendHeartbeat sends a heartbeat to the manager
func (r *Runner) SendHeartbeat() error {
	payload := HeartbeatPayload{
		RunnerID:  r.ID,
		Status:    "active",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat payload: %w", err)
	}

	resp, err := r.client.Post(r.HeartbeatURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	return nil
}

// StartHeartbeat starts periodic heartbeat in a goroutine
func (r *Runner) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := r.SendHeartbeat(); err != nil {
				// Log error but continue heartbeat
				fmt.Printf("Heartbeat error: %v\n", err)
			}
		}
	}()
}

// FetchDeploymentTask fetches a deployment task for the given bin name
func (r *Runner) FetchDeploymentTask(binName string) (*BinDeployment, error) {
	url := fmt.Sprintf("%s/%s", r.BinURL, binName)

	resp, err := r.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch deployment task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch deployment with status: %d", resp.StatusCode)
	}

	var deployment BinDeployment
	if err := json.NewDecoder(resp.Body).Decode(&deployment); err != nil {
		return nil, fmt.Errorf("failed to decode deployment response: %w", err)
	}

	return &deployment, nil
}

// CreateTemplateFromDeployment creates a semaphore template from deployment task
func (r *Runner) CreateTemplateFromDeployment(deployment *BinDeployment) (*db.Template, error) {
	template := &db.Template{
		ProjectID: r.projectID,
		Name:      fmt.Sprintf("deployment-%s", deployment.BinName),
		Playbook:  "deploy.yml",
		Type:      db.TemplateTask,
		App:       db.AppBash, // Use bash app for deployment commands
	}

	// Create template using store
	createdTemplate, err := r.store.CreateTemplate(*template)
	if err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	return &createdTemplate, nil
}

// ExecuteDeployment creates and starts a task based on the deployment
func (r *Runner) ExecuteDeployment(binName string) error {
	// Fetch deployment task from manager
	deployment, err := r.FetchDeploymentTask(binName)
	if err != nil {
		return fmt.Errorf("failed to fetch deployment: %w", err)
	}

	// Create template from deployment
	template, err := r.CreateTemplateFromDeployment(deployment)
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	// Create and start task
	task := &db.Task{
		TemplateID: template.ID,
		ProjectID:  r.projectID,
		Status:     task_logger.TaskWaitingStatus,
	}

	createdTask, err := r.store.CreateTask(*task, 0) // 0 means no limit on max tasks
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	// TODO: Add task to pool for execution
	// This would require access to TaskPool service
	_ = createdTask

	return nil
}
