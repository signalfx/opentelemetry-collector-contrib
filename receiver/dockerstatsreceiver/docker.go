// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dockerstatsreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	dtypes "github.com/docker/docker/api/types"
	dfilters "github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"go.uber.org/zap"
)

const (
	dockerAPIVersion = "v1.22"
	userAgent        = "OpenTelemetry-Collector Docker Stats Receiver/v0.0.1"
)

type DockerClient struct {
	client         *docker.Client
	config         *Config
	containers     map[string]DockerContainer
	containersLock sync.Mutex
	// Store of ContainerJSON.
	inspectedContainers  *InspectedContainers
	excludedImageMatcher *StringMatcher
	logger               *zap.Logger
}

type InspectedContainers struct {
	containers map[string]*dtypes.ContainerJSON
	lock       sync.Mutex
}

func (dc *DockerClient) Containers() []DockerContainer {
	dc.containersLock.Lock()
	defer dc.containersLock.Unlock()
	containers := make([]DockerContainer, 0, len(dc.containers))
	for _, container := range dc.containers {
		containers = append(containers, container)
	}
	return containers
}

func NewDockerClient(config *Config, logger *zap.Logger) (*DockerClient, error) {
	client, err := docker.NewClientWithOpts(
		docker.WithHost(config.Endpoint),
		docker.WithVersion(dockerAPIVersion),
		docker.WithHTTPHeaders(map[string]string{"User-Agent": userAgent}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create docker client: %w", err)
	}

	excludedImageMatcher, err := NewStringMatcher(config.ExcludedImages)
	if err != nil {
		return nil, err
	}

	dc := &DockerClient{
		client:               client,
		config:               config,
		logger:               logger,
		containers:           map[string]DockerContainer{},
		containersLock:       sync.Mutex{},
		excludedImageMatcher: excludedImageMatcher,
		inspectedContainers: &InspectedContainers{
			containers: make(map[string]*dtypes.ContainerJSON),
			lock:       sync.Mutex{},
		},
	}

	return dc, nil
}

func (dc *DockerClient) FetchContainerStatsAndConvertToMetrics(
	ctx context.Context,
	container DockerContainer,
	results chan<- Result,
) {
	dc.logger.Debug("Fetching container stats.", zap.String("id", container.ID))
	statsCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	containerStats, err := dc.client.ContainerStats(statsCtx, container.ID, false)
	defer cancel()
	if err != nil {
		if docker.IsErrNotFound(err) {
			dc.logger.Debug(
				"Daemon reported container doesn't exist. Will no longer monitor.",
				zap.String("id", container.ID),
			)
			dc.removeContainer(container.ID)
		} else {
			dc.logger.Warn(
				"Could not fetch docker containerStats for container",
				zap.String("id", container.ID),
				zap.Error(err),
			)
		}

		results <- Result{nil, err}
		return
	}

	statsJSON, err := dc.toStatsJSON(containerStats, &container, results)
	if err != nil { // results have been sent in converter
		return
	}

	md, err := ContainerStatsToMetrics(statsJSON, &container, dc.config)
	if err != nil {
		dc.logger.Error(
			"Could not convert docker containerStats for container id",
			zap.String("id", container.ID),
			zap.Error(err),
		)
		results <- Result{nil, err}
		return
	}

	results <- Result{md, nil}
}

func (dc *DockerClient) toStatsJSON(
	containerStats dtypes.ContainerStats,
	container *DockerContainer,
	results chan<- Result,
) (*dtypes.StatsJSON, error) {
	var statsJSON dtypes.StatsJSON
	err := json.NewDecoder(containerStats.Body).Decode(&statsJSON)
	containerStats.Body.Close()
	if err != nil {
		// EOF means there aren't any containerStats, perhaps because the container has been removed.
		if err == io.EOF {
			// It isn't indicative of actual error.
			results <- Result{nil, nil}
			return nil, err
		}
		dc.logger.Error(
			"Could not parse docker containerStats for container id",
			zap.String("id", container.ID),
			zap.Error(err),
		)
		results <- Result{nil, err}
		return nil, err
	}
	return &statsJSON, nil
}

// StartWatchingContainers will load the initial running container maps for
// inspection, internal event handling, and establishing which containers
// warrant stat gathering calls by the receiver.  It then initializes the
// main event loop for monitoring.
func (dc *DockerClient) StartWatchingContainers(ctx context.Context) error {
	// Build initial container maps before starting loop
	filters := dfilters.NewArgs()
	filters.Add("status", "running")
	options := dtypes.ContainerListOptions{
		Filters: filters,
	}

	listCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	containerList, err := dc.client.ContainerList(listCtx, options)
	defer cancel()
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for _, c := range containerList {
		wg.Add(1)
		go func(container dtypes.Container) {
			if !dc.shouldBeExcluded(container.Image) {
				dc.inspectedContainers.lock.Lock()
				if dc.updateInspectedContainers(ctx, container.ID) {
					dc.containersLock.Lock()
					dc.updateContainerToQuery(nil, dc.inspectedContainers.containers[container.ID])
					dc.containersLock.Unlock()
				}
				dc.inspectedContainers.lock.Unlock()
			} else {
				dc.logger.Debug(
					"Not monitoring container per ExcludedImages",
					zap.String("image", container.Image),
					zap.String("id", container.ID),
				)
			}
			wg.Done()
		}(c)
	}
	wg.Wait()

	// Start loop
	isRunning := make(chan bool)
	dc.logger.Debug("Starting monitoring for docker_stats receiver.")
	go dc.ContainerEventLoop(ctx, isRunning)
	<-isRunning

	return nil
}

func (dc *DockerClient) ContainerEventLoop(ctx context.Context, hasStarted chan bool) {
	filters := dfilters.NewArgs([]dfilters.KeyValuePair{
		{Key: "type", Value: "container"},
		{Key: "event", Value: "destroy"},
		{Key: "event", Value: "die"},
		{Key: "event", Value: "pause"},
		{Key: "event", Value: "stop"},
		{Key: "event", Value: "start"},
		{Key: "event", Value: "unpause"},
		{Key: "event", Value: "update"},
	}...)
	lastTime := time.Now()
	started := false

EVENT_LOOP:
	for {
		options := dtypes.EventsOptions{
			Filters: filters,
			Since:   lastTime.Format(time.RFC3339Nano),
		}
		eventCh, errCh := dc.client.Events(ctx, options)

		if !started {
			close(hasStarted) // signal to loop initializer
			started = true
		}

		for {
			select {
			case event := <-eventCh:
				dc.inspectedContainers.lock.Lock()
				switch event.Action {
				case "destroy":
					dc.logger.Debug("Docker container was destroyed:", zap.String("id", event.ID))
					dc.containersLock.Lock()
					dc.removeContainer(event.ID)
					dc.containersLock.Unlock()
				default:
					dc.logger.Debug(
						"Docker container update:",
						zap.String("id", event.ID),
						zap.String("action", event.Action),
					)

					existingContainer := dc.inspectedContainers.containers[event.ID]
					if dc.updateInspectedContainers(ctx, event.ID) {
						dc.containersLock.Lock()
						dc.updateContainerToQuery(existingContainer, dc.inspectedContainers.containers[event.ID])
						dc.containersLock.Unlock()
					}
				}
				dc.inspectedContainers.lock.Unlock()

				lastTime = time.Unix(0, event.TimeNano)
			case err := <-errCh:
				dc.logger.Error("Error watching docker container events", zap.Error(err))
				time.Sleep(3 * time.Second)
				continue EVENT_LOOP
			case <-ctx.Done():
				return
			}
		}
	}
}

// Queries inspect api and updates inspectedContainers store.  Returns true when successful,
// and updates to main container store are necessary (dc.updateContainerToQuery())
// You must have claimed inspectedContainers.lock.
func (dc *DockerClient) updateInspectedContainers(ctx context.Context, cid string) bool {
	inspectCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	container, err := dc.client.ContainerInspect(inspectCtx, cid)
	defer cancel()
	if err != nil {
		dc.logger.Error(
			"Could not inspect updated container",
			zap.String("id", cid),
			zap.Error(err),
		)
	} else if !dc.shouldBeExcluded(container.Config.Image) {
		dc.inspectedContainers.containers[cid] = &container
		return true
	}
	return false
}

// Will propagate changes obtained in event loop to main container store.
// You must have claimed containersLock.
func (dc *DockerClient) updateContainerToQuery(existing *dtypes.ContainerJSON, new *dtypes.ContainerJSON) {
	if existing == nil && new == nil {
		return
	}

	var cid string
	if new != nil {
		cid = new.ID
	} else {
		cid = existing.ID
	}

	if new == nil || (!new.State.Running || new.State.Paused) {
		dc.removeContainer(cid)
		return
	}

	dc.logger.Debug("Monitoring Docker container", zap.String("id", cid))

	dc.containers[cid] = DockerContainer{
		ContainerJSON: new,
		EnvMap:        containerEnvToMap(new.Config.Env),
	}
}

// In cases where the destroy event isn't emitted we need
// to forcibly clear container records to prevent repeated
// ContainerStats calls that are destined to fail.
// You must have claimed inspectedContainers.lock and containersLock
func (dc *DockerClient) removeContainer(cid string) {
	delete(dc.inspectedContainers.containers, cid)
	delete(dc.containers, cid)
	dc.logger.Debug("Removed container from stores.", zap.String("id", cid))
}

func (dc *DockerClient) shouldBeExcluded(image string) bool {
	return dc.excludedImageMatcher != nil && dc.excludedImageMatcher.Matches(image)
}

func containerEnvToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, v := range env {
		parts := strings.Split(v, "=")
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}
