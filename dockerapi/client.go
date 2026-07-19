package dockerapi

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
	"strconv"
	"strings"
)

// Container is the small subset of Docker's container JSON used by crontab.
type Container struct {
	ID     string            `json:"Id"`
	Labels map[string]string `json:"Labels"`
}

// Event is the small subset of Docker's event JSON used by crontab.
type Event struct {
	ID     string     `json:"id"`
	Status string     `json:"status"`
	Action string     `json:"Action"`
	Actor  EventActor `json:"Actor"`
}

type EventActor struct {
	ID         string            `json:"ID"`
	Attributes map[string]string `json:"Attributes"`
}

// Normalize fills compatibility fields that vary across Docker API versions.
func (e *Event) Normalize() {
	if e.Action == "" {
		e.Action = e.Status
	}
	if e.ID == "" {
		e.ID = e.Actor.ID
	}
	if e.Actor.Attributes == nil {
		e.Actor.Attributes = map[string]string{}
	}
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewFromEnv() (*Client, error) {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	if strings.HasPrefix(host, "unix://") {
		socketPath := strings.TrimPrefix(host, "unix://")
		if socketPath == "" {
			return nil, fmt.Errorf("empty unix Docker socket path")
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		}
		return &Client{httpClient: &http.Client{Transport: transport}, baseURL: "http://docker"}, nil
	}

	if strings.HasPrefix(host, "tcp://") {
		host = "http://" + strings.TrimPrefix(host, "tcp://")
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return &Client{httpClient: http.DefaultClient, baseURL: strings.TrimRight(host, "/")}, nil
	}

	return nil, fmt.Errorf("unsupported DOCKER_HOST: %s", host)
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) ContainerList(ctx context.Context, all bool) ([]Container, error) {
	values := url.Values{}
	if all {
		values.Set("all", "1")
	}
	var containers []Container
	err := c.doJSON(ctx, http.MethodGet, "/containers/json?"+values.Encode(), nil, http.StatusOK, &containers)
	return containers, err
}

func (c *Client) ContainerStart(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil, http.StatusNoContent, http.StatusNotModified)
}

func (c *Client) ContainerStop(ctx context.Context, id string, timeoutSeconds int) error {
	values := url.Values{}
	values.Set("t", strconv.Itoa(timeoutSeconds))
	return c.doNoBody(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop?"+values.Encode(), nil, http.StatusNoContent, http.StatusNotModified)
}

func (c *Client) ContainerRestart(ctx context.Context, id string, timeoutSeconds int) error {
	values := url.Values{}
	values.Set("t", strconv.Itoa(timeoutSeconds))
	return c.doNoBody(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/restart?"+values.Encode(), nil, http.StatusNoContent)
}

func (c *Client) Events(ctx context.Context, filterEvents []string) (<-chan Event, <-chan error) {
	eventCh := make(chan Event)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		filters := map[string][]string{"event": filterEvents}
		encoded, err := json.Marshal(filters)
		if err != nil {
			errCh <- err
			return
		}
		values := url.Values{}
		values.Set("filters", string(encoded))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/events?"+values.Encode(), nil)
		if err != nil {
			errCh <- err
			return
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			errCh <- c.statusError(resp)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var event Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				errCh <- err
				return
			}
			event.Normalize()
			select {
			case eventCh <- event:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return eventCh, errCh
}

func (c *Client) doJSON(ctx context.Context, method, path string, body io.Reader, expected int, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expected {
		return c.statusError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) doNoBody(ctx context.Context, method, path string, body io.Reader, expected ...int) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	for _, code := range expected {
		if resp.StatusCode == code {
			return nil
		}
	}
	return c.statusError(resp)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

func (c *Client) statusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("docker API %s returned %s: %s", resp.Request.URL.Path, resp.Status, strings.TrimSpace(string(body)))
}
