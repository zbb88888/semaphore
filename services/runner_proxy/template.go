package runnerproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/semaphoreui/semaphore/db"
)

// TemplateResponse represents the response from template operations
type TemplateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CreateTemplate creates a new template in Semaphore
// POST /api/v1/projects/{project_id}/templates
func (rp *RunnerProxy) CreateTemplate(projectID int, template db.Template) (*db.Template, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/templates", rp.config.SemaphoreURL, projectID)

	// 确保 ProjectID 被设置
	template.ProjectID = projectID

	jsonData, err := json.Marshal(template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal template: %w", err)
	}

	resp, err := rp.config.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to post template: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("template creation failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 尝试直接解析为 db.Template
	var createdTemplate db.Template
	if err := json.Unmarshal(body, &createdTemplate); err == nil {
		return &createdTemplate, nil
	}

	// 如果失败，尝试解析为 Response 格式
	var templateResp TemplateResponse
	if err := json.Unmarshal(body, &templateResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template response (body: %s): %w", string(body), err)
	}

	// 从 Response.Data 中提取 template
	if templateResp.Data != nil {
		dataBytes, err := json.Marshal(templateResp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		var resultTemplate db.Template
		if err := json.Unmarshal(dataBytes, &resultTemplate); err != nil {
			return nil, fmt.Errorf("failed to unmarshal template from response data: %w", err)
		}

		return &resultTemplate, nil
	}

	return nil, fmt.Errorf("no template data in response")
}

// GetTemplate retrieves a template by ID
// GET /api/v1/projects/{project_id}/templates/{template_id}
func (rp *RunnerProxy) GetTemplate(projectID, templateID int) (*db.Template, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/templates/%d", rp.config.SemaphoreURL, projectID, templateID)

	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get template failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 尝试直接解析为 db.Template
	var template db.Template
	if err := json.Unmarshal(body, &template); err == nil {
		return &template, nil
	}

	// 如果失败，尝试解析为 Response 格式
	var templateResp TemplateResponse
	if err := json.Unmarshal(body, &templateResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template response (body: %s): %w", string(body), err)
	}

	// 从 Response.Data 中提取 template
	if templateResp.Data != nil {
		dataBytes, err := json.Marshal(templateResp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		var resultTemplate db.Template
		if err := json.Unmarshal(dataBytes, &resultTemplate); err != nil {
			return nil, fmt.Errorf("failed to unmarshal template from response data: %w", err)
		}

		return &resultTemplate, nil
	}

	return nil, fmt.Errorf("no template data in response")
}

// ListTemplates retrieves all templates for a project
// GET /api/v1/projects/{project_id}/templates
func (rp *RunnerProxy) ListTemplates(projectID int) ([]db.Template, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/templates", rp.config.SemaphoreURL, projectID)

	resp, err := rp.config.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list templates failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 尝试直接解析为 []db.Template
	var templates []db.Template
	if err := json.Unmarshal(body, &templates); err == nil {
		return templates, nil
	}

	// 如果失败，尝试解析为 Response 格式
	var templateResp TemplateResponse
	if err := json.Unmarshal(body, &templateResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal templates response (body: %s): %w", string(body), err)
	}

	// 从 Response.Data 中提取 templates
	if templateResp.Data != nil {
		dataBytes, err := json.Marshal(templateResp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		var resultTemplates []db.Template
		if err := json.Unmarshal(dataBytes, &resultTemplates); err != nil {
			return nil, fmt.Errorf("failed to unmarshal templates from response data: %w", err)
		}

		return resultTemplates, nil
	}

	return nil, fmt.Errorf("no templates data in response")
}

// UpdateTemplate updates an existing template
// PUT /api/v1/projects/{project_id}/templates/{template_id}
func (rp *RunnerProxy) UpdateTemplate(projectID int, template db.Template) (*db.Template, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d/templates/%d", rp.config.SemaphoreURL, projectID, template.ID)

	// 确保 ProjectID 被设置
	template.ProjectID = projectID

	jsonData, err := json.Marshal(template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal template: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rp.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("template update failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 尝试直接解析为 db.Template
	var updatedTemplate db.Template
	if err := json.Unmarshal(body, &updatedTemplate); err == nil {
		return &updatedTemplate, nil
	}

	// 如果失败，尝试解析为 Response 格式
	var templateResp TemplateResponse
	if err := json.Unmarshal(body, &templateResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template response (body: %s): %w", string(body), err)
	}

	// 从 Response.Data 中提取 template
	if templateResp.Data != nil {
		dataBytes, err := json.Marshal(templateResp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		var resultTemplate db.Template
		if err := json.Unmarshal(dataBytes, &resultTemplate); err != nil {
			return nil, fmt.Errorf("failed to unmarshal template from response data: %w", err)
		}

		return &resultTemplate, nil
	}

	return nil, fmt.Errorf("no template data in response")
}

// DeleteTemplate deletes a template
// DELETE /api/v1/projects/{project_id}/templates/{template_id}
func (rp *RunnerProxy) DeleteTemplate(projectID, templateID int) error {
	url := fmt.Sprintf("%s/api/v1/projects/%d/templates/%d", rp.config.SemaphoreURL, projectID, templateID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	resp, err := rp.config.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("template deletion failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
