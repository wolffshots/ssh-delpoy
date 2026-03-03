package komodo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps HTTP communication with Komodo API.
type Client struct {
	addr      string
	apiKey    string
	apiSecret string
	httpCli   *http.Client
}

// NewClient creates a new Komodo API client.
func NewClient(addr, apiKey, apiSecret string) *Client {
	return &Client{
		addr:      addr,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		httpCli: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Error represents a Komodo API error response.
type Error struct {
	Error string `json:"error"`
	Trace string `json:"trace,omitempty"`
}

// --------------- EXECUTE REQUESTS ---------------

// PullStack represents a `docker compose pull` execution.
type PullStack struct {
	Stack    string   `json:"stack"`
	Services []string `json:"services"`
}

// DeployStack represents a `docker compose up` execution.
type DeployStack struct {
	Stack    string   `json:"stack"`
	Services []string `json:"services"`
	StopTime *int32   `json:"stop_time,omitempty"`
}

// DestroyStack represents a `docker compose down` execution.
type DestroyStack struct {
	Stack         string   `json:"stack"`
	Services      []string `json:"services"`
	RemoveOrphans bool     `json:"remove_orphans"`
	StopTime      *int32   `json:"stop_time,omitempty"`
}

// --------------- READ REQUESTS ---------------

// ListStackServices lists services in a stack.
type ListStackServices struct {
	Stack string `json:"stack"`
}

// GetStackLog fetches logs from a stack.
type GetStackLog struct {
	Stack      string   `json:"stack"`
	Services   []string `json:"services"`
	Tail       uint64   `json:"tail"`
	Timestamps bool     `json:"timestamps"`
}

// GetStackActionState fetches current action state.
type GetStackActionState struct {
	Stack string `json:"stack"`
}

// --------------- RESPONSES ---------------

// Update represents a long-running update response.
type Update struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// Additional fields may be present in response
}

// StackService represents a service in a stack.
type StackService struct {
	Name  string `json:"name"`
	State string `json:"state"`
	// Additional fields may be present
}

// StackActionState represents action state for a stack.
type StackActionState struct {
	ActionID string `json:"action_id,omitempty"`
	Status   string `json:"status"`
	// Additional fields may be present
}

// Log represents stack logs.
type Log struct {
	Output string `json:"output"`
	// Additional fields may be present
}

// --------------- HTTP HELPERS ---------------

// doRequest sends a JSON request to Komodo and returns the raw response body.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.addr+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("X-Api-Secret", c.apiSecret)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp Error
		_ = json.Unmarshal(respBody, &errResp)
		if errResp.Error != "" {
			return nil, fmt.Errorf("komodo error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// --------------- EXECUTE OPERATIONS ---------------

// ExecutePullStack executes a pull operation.
func (c *Client) ExecutePullStack(ctx context.Context, stack string, services []string) (*Update, error) {
	req := PullStack{
		Stack:    stack,
		Services: services,
	}
	body := map[string]interface{}{
		"type":   "PullStack",
		"params": req,
	}

	resp, err := c.doRequest(ctx, "POST", "/execute", body)
	if err != nil {
		return nil, err
	}

	var update Update
	if err := json.Unmarshal(resp, &update); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &update, nil
}

// ExecuteDeployStack executes a deploy operation.
func (c *Client) ExecuteDeployStack(ctx context.Context, stack string, services []string, stopTime *int32) (*Update, error) {
	req := DeployStack{
		Stack:    stack,
		Services: services,
		StopTime: stopTime,
	}
	body := map[string]interface{}{
		"type":   "DeployStack",
		"params": req,
	}

	resp, err := c.doRequest(ctx, "POST", "/execute", body)
	if err != nil {
		return nil, err
	}

	var update Update
	if err := json.Unmarshal(resp, &update); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &update, nil
}

// ExecuteDestroyStack executes a destroy operation.
func (c *Client) ExecuteDestroyStack(ctx context.Context, stack string, services []string, removeOrphans bool, stopTime *int32) (*Update, error) {
	req := DestroyStack{
		Stack:         stack,
		Services:      services,
		RemoveOrphans: removeOrphans,
		StopTime:      stopTime,
	}
	body := map[string]interface{}{
		"type":   "DestroyStack",
		"params": req,
	}

	resp, err := c.doRequest(ctx, "POST", "/execute", body)
	if err != nil {
		return nil, err
	}

	var update Update
	if err := json.Unmarshal(resp, &update); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &update, nil
}

// --------------- READ OPERATIONS ---------------

// ReadListStackServices lists services in a stack.
func (c *Client) ReadListStackServices(ctx context.Context, stack string) ([]StackService, error) {
	req := ListStackServices{
		Stack: stack,
	}
	body := map[string]interface{}{
		"type":   "ListStackServices",
		"params": req,
	}

	resp, err := c.doRequest(ctx, "POST", "/read", body)
	if err != nil {
		return nil, err
	}

	var services []StackService
	if err := json.Unmarshal(resp, &services); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return services, nil
}

// ReadGetStackLog fetches logs from a stack.
func (c *Client) ReadGetStackLog(ctx context.Context, stack string, services []string, tail uint64, timestamps bool) (*Log, error) {
	req := GetStackLog{
		Stack:      stack,
		Services:   services,
		Tail:       tail,
		Timestamps: timestamps,
	}
	body := map[string]interface{}{
		"type":   "GetStackLog",
		"params": req,
	}

	resp, err := c.doRequest(ctx, "POST", "/read", body)
	if err != nil {
		return nil, err
	}

	var log Log
	if err := json.Unmarshal(resp, &log); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &log, nil
}

// ReadGetStackActionState fetches action state for a stack.
func (c *Client) ReadGetStackActionState(ctx context.Context, stack string) (*StackActionState, error) {
	req := GetStackActionState{
		Stack: stack,
	}
	body := map[string]interface{}{
		"type":   "GetStackActionState",
		"params": req,
	}

	resp, err := c.doRequest(ctx, "POST", "/read", body)
	if err != nil {
		return nil, err
	}

	var state StackActionState
	if err := json.Unmarshal(resp, &state); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &state, nil
}
