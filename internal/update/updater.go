package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Config holds update system configuration.
type Config struct {
	CurrentVersion string
	GitHubRepo     string // "owner/repo"
	DataDir        string
	ServerImage    string // Docker Hub image, e.g. "desent/server"
	WebImage       string // Docker Hub image, e.g. "desent/web"
	ComposeProject string // compose project name for label filtering
}

// ProgressPhase represents the current phase of an update operation.
type ProgressPhase string

const (
	PhaseIdle        ProgressPhase = "idle"
	PhaseChecking    ProgressPhase = "checking"
	PhaseDownloading ProgressPhase = "downloading"
	PhaseApplying    ProgressPhase = "applying"
	PhaseRestarting  ProgressPhase = "restarting"
	PhaseFailed      ProgressPhase = "failed"
	PhaseComplete    ProgressPhase = "complete"
)

// Progress represents the current state of an update operation.
type Progress struct {
	Phase           ProgressPhase `json:"phase"`
	Percent         int           `json:"percent"`
	BytesDownloaded int64         `json:"bytes_downloaded"`
	BytesTotal      int64         `json:"bytes_total"`
	Message         string        `json:"message"`
	Error           string        `json:"error,omitempty"`
}

// ReleaseInfo holds info about a GitHub release.
type ReleaseInfo struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"published_at"`
	ReleaseURL  string    `json:"release_url"`
	Body        string    `json:"body"`
}

// UpdateCheckResult is the response for the check endpoint.
type UpdateCheckResult struct {
	CurrentVersion  string       `json:"current_version"`
	LatestVersion   string       `json:"latest_version"`
	UpdateAvailable bool         `json:"update_available"`
	Release         *ReleaseInfo `json:"release,omitempty"`
	SocketAvailable bool         `json:"socket_available"`
}

// Updater manages the self-update lifecycle.
type Updater struct {
	cfg         Config
	docker      *DockerClient
	mu          sync.Mutex
	updating    bool
	progress    Progress
	subscribers map[chan Progress]struct{}
	subMu       sync.Mutex
	cachedCheck *UpdateCheckResult
	cacheExpiry time.Time
}

// NewUpdater creates a new Updater.
func NewUpdater(cfg Config) *Updater {
	return &Updater{
		cfg:         cfg,
		docker:      newDockerClient(),
		progress:    Progress{Phase: PhaseIdle},
		subscribers: make(map[chan Progress]struct{}),
	}
}

// Subscribe returns a channel that receives progress updates.
func (u *Updater) Subscribe() chan Progress {
	ch := make(chan Progress, 16)
	u.subMu.Lock()
	u.subscribers[ch] = struct{}{}
	u.subMu.Unlock()
	// Send current state immediately.
	u.mu.Lock()
	p := u.progress
	u.mu.Unlock()
	select {
	case ch <- p:
	default:
	}
	return ch
}

// Unsubscribe removes a subscriber channel.
func (u *Updater) Unsubscribe(ch chan Progress) {
	u.subMu.Lock()
	delete(u.subscribers, ch)
	u.subMu.Unlock()
	// Drain and close.
	close(ch)
}

func (u *Updater) broadcast(p Progress) {
	u.mu.Lock()
	u.progress = p
	u.mu.Unlock()

	u.subMu.Lock()
	defer u.subMu.Unlock()
	for ch := range u.subscribers {
		select {
		case ch <- p:
		default:
			// Drop if subscriber is slow.
		}
	}
}

// IsUpdating returns true if an update is in progress.
func (u *Updater) IsUpdating() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.updating
}

// CurrentProgress returns the current progress state.
func (u *Updater) CurrentProgress() Progress {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.progress
}

// githubRelease is the GitHub API response for a release.
type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
}

// CheckForUpdate queries GitHub for the latest release.
func (u *Updater) CheckForUpdate(ctx context.Context) (*UpdateCheckResult, error) {
	u.mu.Lock()
	if u.cachedCheck != nil && time.Now().Before(u.cacheExpiry) {
		result := u.cachedCheck
		u.mu.Unlock()
		return result, nil
	}
	u.mu.Unlock()

	parts := strings.SplitN(u.cfg.GitHubRepo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("update: invalid github repo format: %s", u.cfg.GitHubRepo)
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", parts[0], parts[1])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("update: create github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "desent-updater/"+u.cfg.CurrentVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		result := &UpdateCheckResult{
			CurrentVersion:  u.cfg.CurrentVersion,
			LatestVersion:   u.cfg.CurrentVersion,
			UpdateAvailable: false,
			SocketAvailable: SocketAvailable(),
		}
		return result, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("update: github API status %d: %s", resp.StatusCode, body)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("update: decode github response: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(u.cfg.CurrentVersion, "v")

	result := &UpdateCheckResult{
		CurrentVersion:  u.cfg.CurrentVersion,
		LatestVersion:   release.TagName,
		UpdateAvailable: latestVersion != currentVersion && currentVersion != "dev",
		SocketAvailable: SocketAvailable(),
	}

	if result.UpdateAvailable {
		result.Release = &ReleaseInfo{
			Version:     release.TagName,
			PublishedAt: release.PublishedAt,
			ReleaseURL:  release.HTMLURL,
			Body:        release.Body,
		}
	}

	u.mu.Lock()
	u.cachedCheck = result
	u.cacheExpiry = time.Now().Add(5 * time.Minute)
	u.mu.Unlock()

	return result, nil
}

// Apply starts the update process in a background goroutine.
// Returns an error if update is already in progress or socket is unavailable.
func (u *Updater) Apply(ctx context.Context) error {
	u.mu.Lock()
	if u.updating {
		u.mu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	u.updating = true
	u.mu.Unlock()

	if !SocketAvailable() {
		u.mu.Lock()
		u.updating = false
		u.mu.Unlock()
		return fmt.Errorf("docker socket not available")
	}

	go u.runApply(ctx)
	return nil
}

func (u *Updater) runApply(ctx context.Context) {
	defer func() {
		u.mu.Lock()
		u.updating = false
		u.mu.Unlock()
	}()

	// Get latest version.
	u.broadcast(Progress{Phase: PhaseChecking, Message: "checking for latest version..."})
	check, err := u.CheckForUpdate(ctx)
	if err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to check for update: %v", err)})
		return
	}
	if !check.UpdateAvailable {
		u.broadcast(Progress{Phase: PhaseFailed, Error: "no update available"})
		return
	}

	newTag := check.LatestVersion
	slog.Info("update: starting update", "from", u.cfg.CurrentVersion, "to", newTag)

	// Phase 1: Pull images.
	u.broadcast(Progress{Phase: PhaseDownloading, Message: "pulling server image...", Percent: 0})
	err = u.docker.PullImage(ctx, u.cfg.ServerImage, newTag, func(status string, current, total int64) {
		pct := 0
		if total > 0 {
			pct = int(float64(current) / float64(total) * 40) // 0-40% for server image
		}
		u.broadcast(Progress{
			Phase:           PhaseDownloading,
			Percent:         pct,
			BytesDownloaded: current,
			BytesTotal:      total,
			Message:         fmt.Sprintf("pulling server image: %s", status),
		})
	})
	if err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to pull server image: %v", err)})
		return
	}

	u.broadcast(Progress{Phase: PhaseDownloading, Message: "pulling web image...", Percent: 40})
	err = u.docker.PullImage(ctx, u.cfg.WebImage, newTag, func(status string, current, total int64) {
		pct := 40
		if total > 0 {
			pct = 40 + int(float64(current)/float64(total)*40) // 40-80% for web image
		}
		u.broadcast(Progress{
			Phase:           PhaseDownloading,
			Percent:         pct,
			BytesDownloaded: current,
			BytesTotal:      total,
			Message:         fmt.Sprintf("pulling web image: %s", status),
		})
	})
	if err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to pull web image: %v", err)})
		return
	}

	// Phase 2: Recreate web container.
	u.broadcast(Progress{Phase: PhaseApplying, Percent: 80, Message: "updating web container..."})

	webContainer, err := u.findContainerByService(ctx, "web")
	if err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to find web container: %v", err)})
		return
	}

	newWebImage := u.cfg.WebImage + ":" + newTag
	if err := RecreateContainer(ctx, u.docker, webContainer.ID, newWebImage); err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to update web container: %v", err)})
		return
	}
	slog.Info("update: web container updated", "image", newWebImage)

	// Phase 3: Launch helper container to recreate self.
	u.broadcast(Progress{Phase: PhaseRestarting, Percent: 90, Message: "restarting server..."})

	selfID, err := getSelfContainerID()
	if err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to get own container ID: %v", err)})
		return
	}

	selfInfo, err := u.docker.InspectContainer(ctx, selfID)
	if err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to inspect self: %v", err)})
		return
	}

	selfName := strings.TrimPrefix(selfInfo.Name, "/")
	currentImage := selfInfo.Config.Image
	newServerImage := u.cfg.ServerImage + ":" + newTag

	// Find the entrypoint binary path from the current image.
	entrypoint := "/server"
	if len(selfInfo.Config.Entrypoint) > 0 {
		entrypoint = selfInfo.Config.Entrypoint[0]
	}

	helperName := selfName + "-updater-" + fmt.Sprintf("%d", time.Now().Unix())

	helperBody := ContainerCreateBody{
		Image: currentImage,
		Cmd: []string{
			entrypoint, "update-self",
			"--container", selfID,
			"--image", newServerImage,
			"--name", selfName,
		},
		Labels: map[string]string{
			"desent.updater": "true",
		},
		HostConfig: &HostConfig{
			Binds:      []string{dockerSocket + ":" + dockerSocket},
			AutoRemove: true,
		},
	}

	// Connect helper to the same networks as self.
	if len(selfInfo.NetworkSettings.Networks) > 0 {
		helperBody.NetworkingConfig = &NetworkingConfig{
			EndpointsConfig: make(map[string]EndpointConfig),
		}
		for netName := range selfInfo.NetworkSettings.Networks {
			helperBody.NetworkingConfig.EndpointsConfig[netName] = EndpointConfig{}
			break // Only need one network for connectivity.
		}
	}

	helperID, err := u.docker.CreateContainer(ctx, helperName, helperBody)
	if err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to create helper container: %v", err)})
		return
	}

	if err := u.docker.StartContainer(ctx, helperID); err != nil {
		u.broadcast(Progress{Phase: PhaseFailed, Error: fmt.Sprintf("failed to start helper container: %v", err)})
		return
	}

	slog.Info("update: helper container launched", "id", helperID, "name", helperName)

	// Send final restarting message. The server will be killed by the helper shortly.
	u.broadcast(Progress{
		Phase:   PhaseRestarting,
		Percent: 95,
		Message: "server is restarting with new version...",
	})
}

// findContainerByService finds a container by its compose service label.
func (u *Updater) findContainerByService(ctx context.Context, service string) (*ContainerSummary, error) {
	labels := map[string]string{
		"com.docker.compose.service": service,
	}
	if u.cfg.ComposeProject != "" {
		labels["com.docker.compose.project"] = u.cfg.ComposeProject
	}

	containers, err := u.docker.ListContainers(ctx, labels)
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, fmt.Errorf("no container found for service %q", service)
	}
	return &containers[0], nil
}

// RecreateContainer stops, removes, and recreates a container with a new image.
// It preserves env, ports, volumes, networks, labels, and restart policy.
// On failure after removal, it attempts rollback to the old image.
func RecreateContainer(ctx context.Context, docker *DockerClient, containerID, newImage string) error {
	info, err := docker.InspectContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}

	oldImage := info.Config.Image
	containerName := strings.TrimPrefix(info.Name, "/")

	slog.Info("update: recreating container", "name", containerName, "old_image", oldImage, "new_image", newImage)

	// Stop.
	if err := docker.StopContainer(ctx, containerID, 30); err != nil {
		return fmt.Errorf("stop: %w", err)
	}

	// Remove.
	if err := docker.RemoveContainer(ctx, containerID); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	// Build create body from inspected config.
	body := ContainerCreateBody{
		Image:        newImage,
		Env:          info.Config.Env,
		Cmd:          info.Config.Cmd,
		Entrypoint:   info.Config.Entrypoint,
		ExposedPorts: info.Config.ExposedPorts,
		Labels:       info.Config.Labels,
		WorkingDir:   info.Config.WorkingDir,
		User:         info.Config.User,
		HostConfig: &HostConfig{
			Binds:         info.HostConfig.Binds,
			PortBindings:  info.HostConfig.PortBindings,
			RestartPolicy: info.HostConfig.RestartPolicy,
			NetworkMode:   info.HostConfig.NetworkMode,
		},
	}

	// Preserve network connections.
	if len(info.NetworkSettings.Networks) > 0 {
		body.NetworkingConfig = &NetworkingConfig{
			EndpointsConfig: make(map[string]EndpointConfig),
		}
		for netName, endpoint := range info.NetworkSettings.Networks {
			body.NetworkingConfig.EndpointsConfig[netName] = EndpointConfig{
				Aliases: endpoint.Aliases,
			}
		}
	}

	// Create new container.
	newID, err := docker.CreateContainer(ctx, containerName, body)
	if err != nil {
		slog.Error("update: create failed, attempting rollback", "err", err)
		// Rollback: recreate from old image.
		body.Image = oldImage
		rollbackID, rbErr := docker.CreateContainer(ctx, containerName, body)
		if rbErr != nil {
			return fmt.Errorf("create new container failed (%v) and rollback also failed (%v)", err, rbErr)
		}
		if rbErr := docker.StartContainer(ctx, rollbackID); rbErr != nil {
			return fmt.Errorf("create new container failed (%v) and rollback start failed (%v)", err, rbErr)
		}
		return fmt.Errorf("create new container failed (rolled back to old image): %w", err)
	}

	// Start new container.
	if err := docker.StartContainer(ctx, newID); err != nil {
		slog.Error("update: start failed, attempting rollback", "err", err)
		// Try to remove the new container and rollback.
		_ = docker.RemoveContainer(ctx, newID)
		body.Image = oldImage
		rollbackID, rbErr := docker.CreateContainer(ctx, containerName, body)
		if rbErr != nil {
			return fmt.Errorf("start new container failed (%v) and rollback also failed (%v)", err, rbErr)
		}
		if rbErr := docker.StartContainer(ctx, rollbackID); rbErr != nil {
			return fmt.Errorf("start new container failed (%v) and rollback start failed (%v)", err, rbErr)
		}
		return fmt.Errorf("start new container failed (rolled back to old image): %w", err)
	}

	slog.Info("update: container recreated", "name", containerName, "id", newID)
	return nil
}

// getSelfContainerID returns the container ID of the current process.
// Docker sets the hostname to the container ID.
func getSelfContainerID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("update: get hostname: %w", err)
	}
	if len(hostname) < 12 {
		return "", fmt.Errorf("update: hostname %q doesn't look like a container ID", hostname)
	}
	return hostname, nil
}
