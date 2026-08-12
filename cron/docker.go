package cron

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/PastureStack/container-cron/dockerapi"
	"github.com/sirupsen/logrus"
)

// DockerJob implements the cron job interface
type DockerJob struct {
	mu             sync.RWMutex
	ID             string
	Action         string
	Schedule       string
	Leader         bool
	Labels         map[string]string
	ServiceUUID    string
	Active         bool
	lastError      error
	restartTimeout time.Duration
}

// Err returns last error message
func (dj *DockerJob) Err() error {
	dj.mu.RLock()
	defer dj.mu.RUnlock()
	return dj.lastError
}

// Run Implements the job interface from cron package
func (dj *DockerJob) Run() {
	defer dj.resetErr()

	dj.mu.RLock()
	active, action, id := dj.Active, dj.Action, dj.ID
	dj.mu.RUnlock()
	if active {
		logrus.Debugf("Executing: %s on %s", action, id)
		switch action {
		case "start":
			dj.start()
		case "restart":
			dj.restart()
		case "stop":
			dj.stop()
		default:
			logrus.Errorf("Unsupported action: %s for container id: %s", action, id)
		}
	}

	if dj.Err() != nil {
		logrus.Error(dj.Err())
	}
}

func (dj *DockerJob) resetErr() {
	dj.mu.Lock()
	defer dj.mu.Unlock()
	if dj.lastError != nil {
		logrus.Debugf("Reseting error on %s", dj.ID)
	}
	dj.lastError = nil
}

func (dj *DockerJob) start() {
	dj.withDockerClient(func(client *dockerapi.Client) error {
		return client.ContainerStart(context.Background(), dj.ID)
	})
}

func (dj *DockerJob) restart() {
	dj.withDockerClient(func(client *dockerapi.Client) error {
		return client.ContainerRestart(context.Background(), dj.ID, dockerTimeoutSeconds(dj.restartTimeout))
	})
}

func (dj *DockerJob) stop() {
	dj.withDockerClient(func(client *dockerapi.Client) error {
		return client.ContainerStop(context.Background(), dj.ID, dockerTimeoutSeconds(dj.restartTimeout))
	})
}

func (dj *DockerJob) withDockerClient(action func(*dockerapi.Client) error) {
	client, err := getDockerClient()
	if err == nil {
		defer client.Close()
		err = action(client)
	}
	dj.mu.Lock()
	dj.lastError = err
	dj.mu.Unlock()
}

func getDockerClient() (*dockerapi.Client, error) {
	return dockerapi.NewFromEnv()
}

func dockerTimeoutSeconds(timeout time.Duration) int {
	seconds := int(timeout / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return seconds
}

// NewDockerJob creates a DockerJob and sets defaults
func NewDockerJob(id string, labels map[string]string) *DockerJob {
	dj := &DockerJob{
		ID:             id,
		Schedule:       labels["cron.schedule"],
		Labels:         labels,
		Action:         "start",
		Leader:         false,
		Active:         true,
		lastError:      nil,
		restartTimeout: getDuration(10),
	}

	if value, ok := labels["cron.action"]; ok {
		dj.Action = value
	}

	if _, ok := labels["cron.leader"]; ok {
		dj.Leader = true
	}

	if TO, ok := labels["cron.restart_timeout"]; ok {
		i, err := strconv.Atoi(TO)
		if err != nil {
			logrus.Error("Error converting cron.restart_timeout to int, sticking with default of 10seconds")
			logrus.Error(err)
			i = 10
		}
		dj.restartTimeout = getDuration(i)
	}

	return dj
}

// Deactivate Sets the Actve attribute to false. This will skip running
func (dj *DockerJob) Deactivate() {
	logrus.Debugf("Deactivating: %s", dj.ID)
	dj.mu.Lock()
	defer dj.mu.Unlock()
	dj.Active = false
}

func (dj *DockerJob) Activate() {
	logrus.Debugf("Activating: %s", dj.ID)
	dj.mu.Lock()
	defer dj.mu.Unlock()
	dj.Active = true
}

func (dj *DockerJob) IsActive() bool {
	dj.mu.RLock()
	defer dj.mu.RUnlock()
	return dj.Active
}

func (dj *DockerJob) GetServiceUUID() string {
	dj.mu.RLock()
	defer dj.mu.RUnlock()
	return dj.ServiceUUID
}

func (dj *DockerJob) SetServiceUUID(uuid string) {
	dj.mu.Lock()
	defer dj.mu.Unlock()
	dj.ServiceUUID = uuid
}
