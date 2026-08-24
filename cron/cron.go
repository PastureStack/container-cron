package cron

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PastureStack/container-cron/internal/metadata"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

var newMetadataClient = metadata.NewClientAndWait

type cronJob interface {
	Deactivate()
}

// Crontab is the struct that holds the cron runner
type Crontab struct {
	cronRunner   *cron.Cron
	mu           sync.RWMutex
	jobs         map[string]*JobEntry
	mdClient     metadata.Client
	metadataMode bool
	stopOnce     sync.Once
	stopCh       chan struct{}
}

type JobEntry struct {
	CronID cron.EntryID
	Job    *DockerJob
}

// NewCrontab creates the crontab
func NewCrontab() (*Crontab, error) {
	logrus.Infof("Starting Cron")
	crontab := &Crontab{
		cronRunner: cron.New(cron.WithSeconds()),
		jobs:       map[string]*JobEntry{},
		stopCh:     make(chan struct{}),
	}

	crontab.cronRunner.Start()

	return crontab, nil
}

func NewMetadataAwareCrontab(metadataURL string) (*Crontab, error) {
	crontab, err := NewCrontab()
	if err != nil {
		return nil, err
	}

	crontab.mdClient, err = newMetadataClient(metadataURL)
	if err != nil {
		crontab.Close()
		return nil, err
	}

	crontab.metadataMode = true

	go crontab.watchMetadata()

	return crontab, nil
}

// Close stops scheduled jobs and background metadata polling.
func (ct *Crontab) Close() {
	ct.stopOnce.Do(func() {
		close(ct.stopCh)
		ct.cronRunner.Stop()
	})
}

// GetEntries lists the cron entries
func (ct *Crontab) GetEntries() []cron.Entry {
	entries := ct.cronRunner.Entries()
	return entries
}

// AddJob Adds a docker job to the crontab
func (ct *Crontab) AddJob(id string, labels map[string]string, jobType string) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	var job *DockerJob

	if _, ok := ct.jobs[id]; ok {
		logrus.Debugf("Ignoring Event: %s with job entry: %v", id, ct.jobs[id])
		return nil
	}

	schedule, ok := labels["cron.schedule"]
	if !ok {
		return fmt.Errorf("No cron schedule found for container: %s", id)
	}

	switch jobType {
	case "docker":
		job = NewDockerJob(id, labels)
	default:
		return fmt.Errorf("unknown job type: %s", jobType)
	}

	jobID, err := ct.cronRunner.AddJob(schedule, job)
	if err != nil {
		logrus.Errorf("error adding: %s. Got: %s", id, err)
		return err
	}

	ct.jobs[id] = &JobEntry{
		CronID: jobID,
		Job:    job,
	}

	ct.setJobState(ct.jobs[id])

	logrus.Infof("Added: %s, with schedule: %s", id, schedule)
	return nil
}

// RemoveJob remove a docker job from the cron queue
func (ct *Crontab) RemoveJob(id string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if jobEntry, ok := ct.jobs[id]; ok {
		ct.cronRunner.Remove(jobEntry.CronID)
		delete(ct.jobs, id)
		logrus.Infof("Removed: %s", id)
	}
}

func (ct *Crontab) DeactivateJob(id string, labels map[string]string) error {
	if !ct.metadataMode {
		return nil
	}

	ct.mu.RLock()
	jobEntry, ok := ct.jobs[id]
	ct.mu.RUnlock()
	if ok {
		ct.setJobState(jobEntry)
	}

	return nil
}

func (ct *Crontab) checkMetadataServiceState(uuid string) (string, error) {
	service, err := ct.getServiceByUUID(uuid)
	if err != nil {
		return "", err
	}
	return service.State, nil
}

func (ct *Crontab) getServiceUUID(stackName, serviceName string) (string, error) {
	service, err := ct.getServiceByStackServiceName(stackName, serviceName)
	if err != nil {
		return "", err
	}
	return service.UUID, nil
}

func (ct *Crontab) getServiceByStackServiceName(stackName, serviceName string) (metadata.Service, error) {
	stack, err := ct.mdClient.GetStackByName(stackName)
	if err != nil {
		return metadata.Service{}, err
	}

	for _, service := range stack.Services {
		logrus.Debugf("Comparing %s with %s", service.Name, serviceName)
		if strings.EqualFold(service.Name, serviceName) {
			logrus.Debugf("Returning state: %s for service: %s", service.State, service.Name)
			return service, nil
		}
	}

	return metadata.Service{}, fmt.Errorf("service: %s not found in stack: %s", serviceName, stackName)
}

func (ct *Crontab) getServiceByUUID(uuid string) (metadata.Service, error) {
	services, err := ct.mdClient.GetServices()
	if err != nil {
		return metadata.Service{}, err
	}

	for _, service := range services {
		if service.UUID == uuid {
			return service, nil
		}
	}

	return metadata.Service{}, fmt.Errorf("service with uuid: %s not found", uuid)
}

func (ct *Crontab) watchMetadata() {
	ticker := time.NewTicker(getDuration(5))
	defer ticker.Stop()
	for {
		logrus.Debug("Scanning metadata")
		ct.mu.RLock()
		jobs := make([]*JobEntry, 0, len(ct.jobs))
		for _, job := range ct.jobs {
			jobs = append(jobs, job)
		}
		ct.mu.RUnlock()
		for _, job := range jobs {
			ct.setJobState(job)
		}
		select {
		case <-ct.stopCh:
			return
		case <-ticker.C:
		}
	}
}

func (ct *Crontab) setJobState(job *JobEntry) {
	if !ct.metadataMode {
		return
	}

	serviceUUID := job.Job.GetServiceUUID()
	if serviceUUID == "" {
		stackName := getStackNameFromLabels(job.Job.Labels)

		// A sidekick service label can look like "mainname/sidekickname".
		// but we only need "sidekickname" for this to work in the sidekick case
		serviceStackName := strings.Split(getServiceNameFromLabels(job.Job.Labels), "/")
		serviceName := serviceStackName[len(serviceStackName)-1]
		var err error
		serviceUUID, err = ct.getServiceUUID(stackName, serviceName)
		if err != nil {
			logrus.Error(err)
			return
		}
		job.Job.SetServiceUUID(serviceUUID)
	}

	state, err := ct.checkMetadataServiceState(serviceUUID)
	if err != nil {
		logrus.Error(err)
		return
	}

	// if the job is inactive...activate
	if state == "active" && !job.Job.IsActive() {
		job.Job.Activate()
	}

	// if the job is active... Deactivate
	if state != "active" && job.Job.IsActive() {
		job.Job.Deactivate()
	}
}

func (ct *Crontab) GetNumberOfActiveJobs() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	var i float64
	for _, job := range ct.jobs {
		if job.Job.IsActive() {
			i++
		}
	}
	return i
}

func (ct *Crontab) GetNumberOfInactiveJobs() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	var i float64
	for _, job := range ct.jobs {
		if !job.Job.IsActive() {
			i++
		}
	}
	return i
}
