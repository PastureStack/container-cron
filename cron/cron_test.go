package cron

import (
	"errors"
	"sync"
	"testing"

	"github.com/PastureStack/container-cron/internal/metadata"
)

func TestMetadataClientErrorIsReturned(t *testing.T) {
	wantErr := errors.New("metadata unavailable")
	originalFactory := newMetadataClient
	newMetadataClient = func(string) (metadata.Client, error) {
		return nil, wantErr
	}
	defer func() { newMetadataClient = originalFactory }()

	crontab, err := NewMetadataAwareCrontab("http://metadata.invalid")
	if crontab != nil {
		crontab.Close()
		t.Fatal("expected no crontab after metadata client failure")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestAddJobRejectsUnknownType(t *testing.T) {
	crontab, err := NewCrontab()
	if err != nil {
		t.Fatal(err)
	}
	defer crontab.Close()

	err = crontab.AddJob("container-1", map[string]string{
		"cron.schedule": "0 * * * * *",
	}, "unknown")
	if err == nil {
		t.Fatal("expected an unknown job type error")
	}
}

func TestJobCountsAreSafeDuringStateChanges(t *testing.T) {
	crontab, err := NewCrontab()
	if err != nil {
		t.Fatal(err)
	}
	defer crontab.Close()

	if err := crontab.AddJob("container-1", map[string]string{
		"cron.schedule": "0 * * * * *",
	}, "docker"); err != nil {
		t.Fatal(err)
	}

	crontab.mu.RLock()
	job := crontab.jobs["container-1"].Job
	crontab.mu.RUnlock()

	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			job.Deactivate()
			job.Activate()
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			_ = crontab.GetNumberOfActiveJobs()
			_ = crontab.GetNumberOfInactiveJobs()
		}
	}()
	workers.Wait()

	if got := crontab.GetNumberOfActiveJobs(); got != 1 {
		t.Fatalf("active jobs = %v, want 1", got)
	}
}

func TestDockerClientFailureDoesNotPanic(t *testing.T) {
	t.Setenv("DOCKER_HOST", "invalid://docker")
	job := NewDockerJob("container-1", map[string]string{
		"cron.schedule": "0 * * * * *",
	})

	job.start()
	if job.Err() == nil {
		t.Fatal("expected Docker client construction to fail")
	}
}
