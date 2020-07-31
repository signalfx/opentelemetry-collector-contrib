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
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	dtypes "github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestInvalidEndpoint(t *testing.T) {
	config := &Config{
		Endpoint: "$notavalidendpoint*",
	}
	cli, err := NewDockerClient(config, zap.NewNop())
	assert.Nil(t, cli)
	require.Error(t, err)
	assert.Equal(t, "could not create docker client: unable to parse docker host `$notavalidendpoint*`", err.Error())
}

func tmpSock(t *testing.T) (net.Listener, string) {
	f, err := ioutil.TempFile(os.TempDir(), "testsock")
	if err != nil {
		t.Fatal(err)
	}
	addr := f.Name()
	os.Remove(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	return listener, addr
}

func expectedConnectError(addr string) string {
	return fmt.Sprintf("Cannot connect to the Docker daemon at unix://%s.", addr)
}

func TestWatchingTimeouts(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedError := expectedConnectError(addr)

	shouldHaveTaken := time.Now().Add(100 * time.Millisecond).UnixNano()

	err = cli.StartWatchingContainers(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedError)

	observed, logs := observer.New(zapcore.WarnLevel)
	cli, err = NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	succeeded := cli.updateInspectedContainers(context.Background(), "SomeContainerId")
	assert.False(t, succeeded)
	assert.Equal(t, 1, len(logs.All()))
	for _, l := range logs.All() {
		assert.Contains(t, l.ContextMap()["error"], expectedError)
	}

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)
}

func TestFetchingTimeouts(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedError := expectedConnectError(addr)

	shouldHaveTaken := time.Now().Add(50 * time.Millisecond).UnixNano()

	observed, logs := observer.New(zapcore.WarnLevel)
	cli, err = NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	resultChan := make(chan Result)
	go cli.FetchContainerStatsAndConvertToMetrics(
		context.Background(),
		DockerContainer{
			ContainerJSON: &dtypes.ContainerJSON{
				ContainerJSONBase: &dtypes.ContainerJSONBase{
					ID: "notARealContainerId",
				},
			},
		},
		resultChan,
	)
	result := <-resultChan

	assert.Nil(t, result.md)
	require.Error(t, result.err)

	assert.Contains(t, result.err.Error(), expectedError)

	assert.Equal(t, 1, len(logs.All()))
	for _, l := range logs.All() {
		assert.Contains(t, l.ContextMap()["error"], expectedError)
	}

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)

}

func TestToStatsJSONErrorHandling(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	dc := &DockerContainer{
		ContainerJSON: &dtypes.ContainerJSON{
			ContainerJSONBase: &dtypes.ContainerJSONBase{
				ID: "notARealContainerId",
			},
		},
	}

	resultChan := make(chan Result)
	// EOF should not signify error
	go func() {
		statsJSON, err := cli.toStatsJSON(
			dtypes.ContainerStats{
				Body: ioutil.NopCloser(strings.NewReader("")),
			}, dc, resultChan,
		)
		assert.Nil(t, statsJSON)
		assert.Equal(t, io.EOF, err)
	}()
	result := <-resultChan
	assert.Nil(t, result.md)
	assert.Nil(t, result.err)

	go func() {
		statsJSON, err := cli.toStatsJSON(
			dtypes.ContainerStats{
				Body: ioutil.NopCloser(strings.NewReader("{\"Networks\": 123}")),
			}, dc, resultChan,
		)
		assert.Nil(t, statsJSON)
		require.Error(t, err)
	}()
	result = <-resultChan
	assert.Nil(t, result.md)
	require.Error(t, result.err)
}
