package dockerapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNetworkClientPinsRequestsToConfiguredEndpoint(t *testing.T) {
	requestHost := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHost <- r.Host
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"Id":"container-1","Labels":{}}]`))
	}))
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", server.URL)
	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if client.baseURL != "http://docker" {
		t.Fatalf("client base URL = %q, want fixed Docker API authority", client.baseURL)
	}

	containers, err := client.ContainerList(context.Background(), false)
	if err != nil {
		t.Fatalf("ContainerList() error = %v", err)
	}
	if len(containers) != 1 || containers[0].ID != "container-1" {
		t.Fatalf("ContainerList() = %#v, want one expected container", containers)
	}
	if got := <-requestHost; got != "docker" {
		t.Fatalf("request Host = %q, want fixed Docker API authority", got)
	}
}

func TestNetworkClientRejectsRedirects(t *testing.T) {
	var redirectedRequests atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(redirectTarget.Close)

	dockerEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/containers/json", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(dockerEndpoint.Close)

	t.Setenv("DOCKER_HOST", dockerEndpoint.URL)
	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	_, err = client.ContainerList(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("ContainerList() error = %v, want redirect rejection", err)
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
}

func TestNetworkClientRejectsEndpointURLComponents(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{name: "credentials", host: "http://user:password@127.0.0.1:2375"},
		{name: "path", host: "http://127.0.0.1:2375/docker-api"},
		{name: "query", host: "http://127.0.0.1:2375?target=elsewhere"},
		{name: "fragment", host: "http://127.0.0.1:2375#fragment"},
		{name: "encoded host", host: "http://127.0.0.1%2fmetadata:2375"},
		{name: "invalid port", host: "http://127.0.0.1:70000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", tt.host)
			if client, err := NewFromEnv(); err == nil {
				_ = client.Close()
				t.Fatalf("NewFromEnv() accepted unsafe endpoint %q", tt.host)
			}
		})
	}
}

func TestUnixDockerHostValidation(t *testing.T) {
	tests := []string{
		"unix://",
		"unix://relative/docker.sock",
		"unix:///var/run/docker.sock?target=elsewhere",
		"unix:///var/run/docker.sock#fragment",
	}
	for _, host := range tests {
		t.Run(host, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", host)
			if client, err := NewFromEnv(); err == nil {
				_ = client.Close()
				t.Fatalf("NewFromEnv() accepted unsafe endpoint %q", host)
			}
		})
	}
}

func TestTCPDockerHostRemainsSupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := client.ContainerList(context.Background(), false); err != nil {
		t.Fatalf("ContainerList() over tcp DOCKER_HOST error = %v", err)
	}
}

func TestDockerHostSchemeIsCaseInsensitive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "TCP://"+strings.TrimPrefix(server.URL, "http://"))
	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := client.ContainerList(context.Background(), false); err != nil {
		t.Fatalf("ContainerList() over uppercase TCP scheme error = %v", err)
	}
}
