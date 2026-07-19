package events

import (
	"context"

	"github.com/PastureStack/container-cron/cron"
	"github.com/PastureStack/container-cron/dockerapi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Handler handles messages
type Handler interface {
	Handle(Message)
}

// Message is a message from an event stream
type Message *dockerapi.Event

// DockerHandler handles docker messages
type DockerHandler struct {
	Crontab *cron.Crontab
}

type DockerHandlerOpts struct {
	MetadataMode bool
	MetadataURL  string
}

// NewDockerHandler returns a docker handler with crontab
func NewDockerHandler(opts *DockerHandlerOpts) (*DockerHandler, error) {
	var crontab *cron.Crontab
	var err error
	if opts.MetadataMode {
		logrus.Infof("Using metadata mode with URL = %s", opts.MetadataURL)
		crontab, err = cron.NewMetadataAwareCrontab(opts.MetadataURL)
	} else {
		crontab, err = cron.NewCrontab()
	}
	if err != nil {
		return nil, err
	}

	dClient, err := dockerapi.NewFromEnv()
	if err != nil {
		crontab.Close()
		return nil, err
	}
	defer dClient.Close()

	containers, err := dClient.ContainerList(context.Background(), true)
	if err != nil {
		crontab.Close()
		return nil, err
	}

	// Scan containers
	logrus.Infof("Scanning for container cron entries")
	for _, container := range containers {
		if _, ok := container.Labels["cron.schedule"]; ok {
			if err := crontab.AddJob(container.ID, container.Labels, "docker"); err != nil {
				logrus.Errorf("failed to add container %s: %v", container.ID, err)
			}
		}
	}

	return &DockerHandler{
		Crontab: crontab,
	}, nil
}

// Handle implements handler interface
func (dh DockerHandler) Handle(msg Message) {
	if msg == nil {
		return
	}
	// Adding a cron.schedule label flags the container for deeper inspection
	// With this service
	if _, ok := msg.Actor.Attributes["cron.schedule"]; ok {
		if msg.Action == "start" || msg.Action == "create" {
			logrus.Debugf("Processing %s event for container: %s", msg.Action, msg.ID)
			if err := dh.Crontab.AddJob(msg.ID, msg.Actor.Attributes, "docker"); err != nil {
				logrus.Errorf("failed to add container %s: %v", msg.ID, err)
			}
		}

		if msg.Action == "stop" || msg.Action == "die" {
			logrus.Debugf("Processing %s event for container: %s", msg.Action, msg.ID)
			if err := dh.Crontab.DeactivateJob(msg.ID, msg.Actor.Attributes); err != nil {
				logrus.Errorf("failed to deactivate container %s: %v", msg.ID, err)
			}
		}

		if msg.Action == "destroy" {
			logrus.Debugf("Processing destroy event for container: %s", msg.ID)
			dh.Crontab.RemoveJob(msg.ID)
		}
	}
}

func (dh DockerHandler) GetJobStats(guage *prometheus.GaugeVec) (*prometheus.GaugeVec, error) {
	guage.With(prometheus.Labels{"state": "active"}).Set(dh.Crontab.GetNumberOfActiveJobs())
	guage.With(prometheus.Labels{"state": "inactive"}).Set(dh.Crontab.GetNumberOfInactiveJobs())
	return guage, nil
}
