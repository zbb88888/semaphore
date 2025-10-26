package runnerproxy

// 1. runner.go 的 Runner 对象会调用 这里的 Task 对象，启动 bin 部署任务
// 2. 一旦任务开始，Task 对象会 跟踪 Task 进度，会与 manager 进行交互，更新任务状态

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/semaphoreui/semaphore/db"
)

// Task represents a runner proxy task that manages deployment execution
type Task struct {
	ID        int
	Status    string
	StartTime time.Time
	EndTime   *time.Time
	Output    []byte
	Error     error

	// Internal state
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
	mutex  sync.RWMutex
}

// NewTask creates a new task instance
func NewTask(id int) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	return &Task{
		ID:        id,
		Status:    "pending",
		StartTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins task execution
func (t *Task) Start(command string, args []string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.Status != "pending" {
		return fmt.Errorf("task %d is not in pending state", t.ID)
	}

	t.Status = "running"
	t.cmd = exec.CommandContext(t.ctx, command, args...)

	// Start command execution in goroutine
	go func() {
		defer func() {
			t.mutex.Lock()
			now := time.Now()
			t.EndTime = &now
			if t.Error != nil {
				t.Status = "failed"
			} else {
				t.Status = "completed"
			}
			t.mutex.Unlock()
		}()

		output, err := t.cmd.CombinedOutput()
		t.mutex.Lock()
		t.Output = output
		t.Error = err
		t.mutex.Unlock()
	}()

	return nil
}

// Stop cancels the running task
func (t *Task) Stop() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.Status != "running" {
		return fmt.Errorf("task %d is not running", t.ID)
	}

	t.cancel()
	t.Status = "cancelled"
	now := time.Now()
	t.EndTime = &now

	return nil
}

// GetStatus returns current task status
func (t *Task) GetStatus() string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.Status
}

// GetOutput returns task output
func (t *Task) GetOutput() []byte {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.Output
}

// UpdateManager updates task status in the database/manager
func (t *Task) UpdateManager(store db.Store) error {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// Update task status in database
	// This would interact with the actual database store
	// Implementation depends on specific schema

	return nil
}
