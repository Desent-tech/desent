package update

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const dockerSocket = "/var/run/docker.sock"

// DockerClient talks to Docker Engine API via Unix socket.
type DockerClient struct {
	httpClient *http.Client
}

// NewDockerClient creates a Docker client for use externally (e.g., update-self helper).
func NewDockerClient() *DockerClient {
	return newDockerClient()
}

func newDockerClient() *DockerClient {
	return &DockerClient{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", dockerSocket)
				},
			},
		},
	}
}

// SocketAvailable checks if Docker socket is accessible.
func SocketAvailable() bool {
	info, err := os.Stat(dockerSocket)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

// ContainerSummary is a subset of Docker's container list response.
type ContainerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	Labels map[string]string `json:"Labels"`
	State  string            `json:"State"`
}

// ContainerInspect holds the full container config from Docker inspect.
type ContainerInspect struct {
	ID              string          `json:"Id"`
	Name            string          `json:"Name"`
	Config          ContainerConfig `json:"Config"`
	HostConfig      HostConfig      `json:"HostConfig"`
	NetworkSettings NetworkSettings `json:"NetworkSettings"`
}

type ContainerConfig struct {
	Image        string              `json:"Image"`
	Env          []string            `json:"Env"`
	Cmd          []string            `json:"Cmd"`
	Entrypoint   []string            `json:"Entrypoint"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	Labels       map[string]string   `json:"Labels"`
	WorkingDir   string              `json:"WorkingDir"`
	User         string              `json:"User"`
}

type HostConfig struct {
	Binds         []string                 `json:"Binds"`
	PortBindings  map[string][]PortBinding `json:"PortBindings"`
	RestartPolicy RestartPolicy            `json:"RestartPolicy"`
	NetworkMode   string                   `json:"NetworkMode"`
	AutoRemove    bool                     `json:"AutoRemove"`
}

type PortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type RestartPolicy struct {
	Name              string `json:"Name"`
	MaximumRetryCount int    `json:"MaximumRetryCount"`
}

type NetworkSettings struct {
	Networks map[string]NetworkEndpoint `json:"Networks"`
}

type NetworkEndpoint struct {
	NetworkID  string   `json:"NetworkID"`
	EndpointID string   `json:"EndpointID"`
	Gateway    string   `json:"Gateway"`
	IPAddress  string   `json:"IPAddress"`
	Aliases    []string `json:"Aliases"`
}

// ContainerCreateBody is the request body for creating a container.
type ContainerCreateBody struct {
	Image            string              `json:"Image"`
	Env              []string            `json:"Env,omitempty"`
	Cmd              []string            `json:"Cmd,omitempty"`
	Entrypoint       []string            `json:"Entrypoint,omitempty"`
	ExposedPorts     map[string]struct{} `json:"ExposedPorts,omitempty"`
	Labels           map[string]string   `json:"Labels,omitempty"`
	WorkingDir       string              `json:"WorkingDir,omitempty"`
	User             string              `json:"User,omitempty"`
	HostConfig       *HostConfig         `json:"HostConfig,omitempty"`
	NetworkingConfig *NetworkingConfig   `json:"NetworkingConfig,omitempty"`
}

type NetworkingConfig struct {
	EndpointsConfig map[string]EndpointConfig `json:"EndpointsConfig,omitempty"`
}

type EndpointConfig struct {
	Aliases []string `json:"Aliases,omitempty"`
}

// pullProgress is a single line from Docker's image pull stream.
type pullProgress struct {
	Status         string         `json:"status"`
	ID             string         `json:"id"`
	ProgressDetail progressDetail `json:"progressDetail"`
	Error          string         `json:"error"`
}

type progressDetail struct {
	Current int64 `json:"current"`
	Total   int64 `json:"total"`
}

// PullImage pulls a Docker image, calling onProgress with download status.
func (c *DockerClient) PullImage(ctx context.Context, image, tag string, onProgress func(status string, current, total int64)) error {
	params := url.Values{}
	params.Set("fromImage", image)
	params.Set("tag", tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/images/create?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("update: create pull request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: pull image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update: pull image %s:%s: status %d: %s", image, tag, resp.StatusCode, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	// Docker pull streams can have large JSON lines with embedded progress
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	var totalBytes int64
	layerProgress := make(map[string]int64)

	for scanner.Scan() {
		var p pullProgress
		if err := json.Unmarshal(scanner.Bytes(), &p); err != nil {
			continue
		}
		if p.Error != "" {
			return fmt.Errorf("update: pull image %s:%s: %s", image, tag, p.Error)
		}
		if onProgress != nil {
			if p.ProgressDetail.Total > 0 {
				layerProgress[p.ID] = p.ProgressDetail.Current
				if strings.HasPrefix(p.Status, "Downloading") {
					totalBytes = p.ProgressDetail.Total
				}
			}
			var currentBytes int64
			for _, v := range layerProgress {
				currentBytes += v
			}
			onProgress(p.Status, currentBytes, totalBytes)
		}
	}

	return scanner.Err()
}

// ListContainers lists containers matching the given label filters.
func (c *DockerClient) ListContainers(ctx context.Context, labels map[string]string) ([]ContainerSummary, error) {
	filters := make(map[string][]string)
	for k, v := range labels {
		filters["label"] = append(filters["label"], k+"="+v)
	}
	filtersJSON, _ := json.Marshal(filters)

	params := url.Values{}
	params.Set("filters", string(filtersJSON))
	params.Set("all", "true")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("update: create list request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: list containers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("update: list containers: status %d: %s", resp.StatusCode, body)
	}

	var containers []ContainerSummary
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("update: decode containers: %w", err)
	}
	return containers, nil
}

// InspectContainer returns full container config.
func (c *DockerClient) InspectContainer(ctx context.Context, id string) (*ContainerInspect, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+id+"/json", nil)
	if err != nil {
		return nil, fmt.Errorf("update: create inspect request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: inspect container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("update: inspect container %s: status %d: %s", id, resp.StatusCode, body)
	}

	var info ContainerInspect
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("update: decode inspect: %w", err)
	}
	return &info, nil
}

// StopContainer stops a container with a graceful timeout.
func (c *DockerClient) StopContainer(ctx context.Context, id string, timeoutSec int) error {
	params := url.Values{}
	params.Set("t", fmt.Sprintf("%d", timeoutSec))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/containers/"+id+"/stop?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("update: create stop request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: stop container: %w", err)
	}
	defer resp.Body.Close()

	// 204 = stopped, 304 = already stopped
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update: stop container %s: status %d: %s", id, resp.StatusCode, body)
	}
	return nil
}

// RemoveContainer removes a container.
func (c *DockerClient) RemoveContainer(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://docker/containers/"+id, nil)
	if err != nil {
		return fmt.Errorf("update: create remove request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: remove container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update: remove container %s: status %d: %s", id, resp.StatusCode, body)
	}
	return nil
}

// createResponse is the response from Docker container create.
type createResponse struct {
	ID       string   `json:"Id"`
	Warnings []string `json:"Warnings"`
}

// CreateContainer creates a new container and returns its ID.
func (c *DockerClient) CreateContainer(ctx context.Context, name string, body ContainerCreateBody) (string, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("update: marshal create body: %w", err)
	}

	params := url.Values{}
	if name != "" {
		params.Set("name", name)
	}

	reqURL := "http://docker/containers/create"
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return "", fmt.Errorf("update: create container request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("update: create container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("update: create container %s: status %d: %s", name, resp.StatusCode, respBody)
	}

	var result createResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("update: decode create response: %w", err)
	}
	return result.ID, nil
}

// StartContainer starts a stopped container.
func (c *DockerClient) StartContainer(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/containers/"+id+"/start", nil)
	if err != nil {
		return fmt.Errorf("update: create start request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: start container: %w", err)
	}
	defer resp.Body.Close()

	// 204 = started, 304 = already started
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update: start container %s: status %d: %s", id, resp.StatusCode, body)
	}
	return nil
}
