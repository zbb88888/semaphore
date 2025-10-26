package runnerproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/semaphoreui/semaphore/db"
)

// TaskResponse represents the response from task operations
type TaskResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CreateTask creates a new task in Semaphore
// POST /api/v1/projects/{project_id}/templates/{template_id}/tasks
func (rp *RunnerProxy) CreateTask(projectID, templateID int, task db.Task) (*db.Task, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/templates/%d/tasks", rp.config.SemaphoreURL, projectID, templateID)

	// 确保必要的字段被设置
	task.ProjectID = projectID
	task.TemplateID = templateID

	jsonData, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task: %w", err)
	}

	resp, err := rp.config.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to post task: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("task creation failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 尝试直接解析为 db.Task
	var createdTask db.Task
	if err := json.Unmarshal(body, &createdTask); err == nil {
		return &createdTask, nil
	}

	// 如果失败，尝试解析为 Response 格式
	var taskResp TaskResponse
	if err := json.Unmarshal(body, &taskResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task response (body: %s): %w", string(body), err)
	}

	// 从 Response.Data 中提取 task
	if taskResp.Data != nil {
		dataBytes, err := json.Marshal(taskResp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		var resultTask db.Task
		if err := json.Unmarshal(dataBytes, &resultTask); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task from response data: %w", err)
		}

		return &resultTask, nil
	}

	return nil, fmt.Errorf("no task data in response")
}

// GetTask retrieves a task by ID
// GET /api/v1/projects/{project_id}/tasks/{task_id}
func (rp *RunnerProxy) GetTask(projectID, taskID int) (*db.Task, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/tasks/%d", rp.config.SemaphoreURL, projectID, taskID)

	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get task failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 尝试直接解析为 db.Task
	var task db.Task
	if err := json.Unmarshal(body, &task); err == nil {
		return &task, nil
	}

	// 如果失败，尝试解析为 Response 格式
	var taskResp TaskResponse
	if err := json.Unmarshal(body, &taskResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task response (body: %s): %w", string(body), err)
	}

	// 从 Response.Data 中提取 task
	if taskResp.Data != nil {
		dataBytes, err := json.Marshal(taskResp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		var resultTask db.Task
		if err := json.Unmarshal(dataBytes, &resultTask); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task from response data: %w", err)
		}

		return &resultTask, nil
	}

	return nil, fmt.Errorf("no task data in response")
}

// GetTaskOutput retrieves task output
// GET /api/v1/projects/{project_id}/tasks/{task_id}/output
func (rp *RunnerProxy) GetTaskOutput(projectID, taskID int) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/tasks/%d/output", rp.config.SemaphoreURL, projectID, taskID)

	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get task output: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get task output failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ListTasks retrieves all tasks for a project
// GET /api/v1/projects/{project_id}/tasks
func (rp *RunnerProxy) ListTasks(projectID int) ([]db.Task, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/tasks", rp.config.SemaphoreURL, projectID)

	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list tasks failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 尝试直接解析为 []db.Task
	var tasks []db.Task
	if err := json.Unmarshal(body, &tasks); err == nil {
		return tasks, nil
	}

	// 如果失败，尝试解析为 Response 格式
	var taskResp TaskResponse
	if err := json.Unmarshal(body, &taskResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks response (body: %s): %w", string(body), err)
	}

	// 从 Response.Data 中提取 tasks
	if taskResp.Data != nil {
		dataBytes, err := json.Marshal(taskResp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		var resultTasks []db.Task
		if err := json.Unmarshal(dataBytes, &resultTasks); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tasks from response data: %w", err)
		}

		return resultTasks, nil
	}

	return nil, fmt.Errorf("no tasks data in response")
}

// StopTask stops a running task
// POST /api/v1/projects/{project_id}/tasks/{task_id}/stop
func (rp *RunnerProxy) StopTask(projectID, taskID int) error {
	url := fmt.Sprintf("%s/api/v1/projects/%d/tasks/%d/stop", rp.config.SemaphoreURL, projectID, taskID)

	resp, err := rp.config.HTTPClient.Post(url, "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to stop task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop task failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ConfirmTask confirms a task (used for approval workflows)
// POST /api/v1/projects/{project_id}/tasks/{task_id}/confirm
func (rp *RunnerProxy) ConfirmTask(projectID, taskID int) error {
	url := fmt.Sprintf("%s/api/v1/projects/%d/tasks/%d/confirm", rp.config.SemaphoreURL, projectID, taskID)

	resp, err := rp.config.HTTPClient.Post(url, "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to confirm task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("confirm task failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteTask deletes a task
// DELETE /api/v1/projects/{project_id}/tasks/{task_id}
func (rp *RunnerProxy) DeleteTask(projectID, taskID int) error {
	url := fmt.Sprintf("%s/api/v1/projects/%d/tasks/%d", rp.config.SemaphoreURL, projectID, taskID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	resp, err := rp.config.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("task deletion failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
