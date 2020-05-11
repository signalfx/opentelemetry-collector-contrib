// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dimensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// DimensionClient sends updates to dimensions to the SignalFx API
// This is a port of https://github.com/signalfx/signalfx-agent/blob/master/pkg/core/writer/dimensions/client.go
// with the only major difference being deduplication of dimension
// updates are currently not done by this port.
type DimensionClient struct {
	sync.RWMutex
	ctx           context.Context
	Token         string
	APIURL        *url.URL
	client        *http.Client
	requestSender *ReqSender
	// How long to wait for property updates to be sent once they are
	// generated.  Any duplicate updates to the same dimension within this time
	// frame will result in the latest property set being sent.  This helps
	// prevent spurious updates that get immediately overwritten by very flappy
	// property generation.
	sendDelay time.Duration
	// Set of dims that have been queued up for sending.  Use map to quickly
	// look up in case we need to replace due to flappy prop generation.
	delayedSet map[DimensionKey]*DimensionUpdate
	// Queue of dimensions to update.  The ordering should never change once
	// put in the queue so no need for heap/priority queue.
	delayedQueue chan *queuedDimension
	// For easier unit testing
	now func() time.Time

	// TODO: Send these counters as internal metrics to SignalFx
	DimensionsCurrentlyDelayed int64
	TotalDimensionsDropped     int64
	// The number of dimension updates that happened to the same dimension
	// within sendDelay.
	TotalFlappyUpdates           int64
	TotalClientError4xxResponses int64
	TotalRetriedUpdates          int64
	TotalInvalidDimensions       int64
	TotalSuccessfulUpdates       int64
	logUpdates                   bool
	logger                       *zap.Logger
}

type queuedDimension struct {
	*DimensionUpdate
	TimeToSend time.Time
}

// NewDimensionClient returns a new client
func NewDimensionClient(token string, apiURL *url.URL, logUpdates bool, logger *zap.Logger) *DimensionClient {
	ctx, _ := context.WithCancel(context.Background())

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:        20,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	sender := NewReqSender(ctx, client, 20, map[string]string{"client": "dimension"})

	return &DimensionClient{
		ctx:           ctx,
		Token:         token,
		APIURL:        apiURL,
		sendDelay:     10 * time.Second,
		delayedSet:    make(map[DimensionKey]*DimensionUpdate),
		delayedQueue:  make(chan *queuedDimension, 10000),
		requestSender: sender,
		client:        client,
		now:           time.Now,
		logger:        logger,
		logUpdates:    logUpdates,
	}
}

// Start the client's processing queue
func (dc *DimensionClient) Start() {
	go dc.processQueue()
}

// acceptDimension to be sent to the API.  This will return fairly quickly and
// won't block. If the buffer is full, the dim update will be dropped.
func (dc *DimensionClient) acceptDimension(dimUpdate *DimensionUpdate) error {
	dc.Lock()
	defer dc.Unlock()

	if delayedDimUpdate := dc.delayedSet[dimUpdate.Key()]; delayedDimUpdate != nil {
		if !reflect.DeepEqual(delayedDimUpdate, dimUpdate) {
			dc.TotalFlappyUpdates++
		}
	} else {
		atomic.AddInt64(&dc.DimensionsCurrentlyDelayed, int64(1))

		dc.delayedSet[dimUpdate.Key()] = dimUpdate
		select {
		case dc.delayedQueue <- &queuedDimension{
			DimensionUpdate: dimUpdate,
			TimeToSend:      dc.now().Add(dc.sendDelay),
		}:
			break
		default:
			dc.TotalDimensionsDropped++
			atomic.AddInt64(&dc.DimensionsCurrentlyDelayed, int64(-1))
			return errors.New("dropped dimension update, propertiesMaxBuffered exceeded")
		}
	}

	return nil
}

func (dc *DimensionClient) processQueue() {
	for {
		select {
		case <-dc.ctx.Done():
			return
		case delayedDimUpdate := <-dc.delayedQueue:
			now := dc.now()
			if now.Before(delayedDimUpdate.TimeToSend) {
				// dims are always in the channel in order of TimeToSend
				time.Sleep(delayedDimUpdate.TimeToSend.Sub(now))
			}

			atomic.AddInt64(&dc.DimensionsCurrentlyDelayed, int64(-1))

			dc.Lock()
			delete(dc.delayedSet, delayedDimUpdate.Key())
			dc.Unlock()

			if err := dc.setPropertiesOnDimension(delayedDimUpdate.DimensionUpdate); err != nil {
				dc.logger.Error(
					"Could not send dimension update",
					zap.Error(err),
					zap.String("dimensionUpdate", delayedDimUpdate.String()),
				)
			}
		}
	}
}

// setPropertiesOnDimension will set custom properties on a specific dimension value.
func (dc *DimensionClient) setPropertiesOnDimension(dimUpdate *DimensionUpdate) error {
	var (
		req *http.Request
		err error
	)

	if dimUpdate.Name == "" || dimUpdate.Value == "" {
		atomic.AddInt64(&dc.TotalInvalidDimensions, int64(1))
		return fmt.Errorf("dimensionUpdate %v is missing Name or value, cannot send", dimUpdate)
	}

	req, err = dc.makePatchRequest(dimUpdate)

	if err != nil {
		return err
	}

	req = req.WithContext(
		context.WithValue(req.Context(), RequestFailedCallbackKey, RequestFailedCallback(func(statusCode int, err error) {
			if statusCode >= 400 && statusCode < 500 && statusCode != 404 {
				atomic.AddInt64(&dc.TotalClientError4xxResponses, int64(1))
				dc.logger.Error(
					"Unable to update dimension, not retrying",
					zap.Error(err),
					zap.String("URL", req.URL.String()),
					zap.String("dimensionUpdate", dimUpdate.String()),
				)

				// Don't retry if it is a 4xx error (except 404) since these
				// imply an input/auth error, which is not going to be remedied
				// by retrying.
				// 404 errors are special because they can occur due to races
				// within the dimension patch endpoint.
				return
			}

			dc.logger.Error(
				"Unable to update dimension, retrying",
				zap.Error(err),
				zap.String("URL", req.URL.String()),
				zap.String("dimensionUpdate", dimUpdate.String()),
			)
			atomic.AddInt64(&dc.TotalRetriedUpdates, int64(1))
			// The retry is meant to provide some measure of robustness against
			// temporary API failures.  If the API is down for significant
			// periods of time, dimension updates will probably eventually back
			// up beyond conf.PropertiesMaxBuffered and start dropping.
			if err := dc.acceptDimension(dimUpdate); err != nil {
				dc.logger.Error(
					"Failed to retry dimension update",
					zap.Error(err),
					zap.String("URL", req.URL.String()),
					zap.String("dimensionUpdate", dimUpdate.String()),
				)
			}
		})))

	req = req.WithContext(
		context.WithValue(req.Context(), RequestSuccessCallbackKey, RequestSuccessCallback(func([]byte) {
			if dc.logUpdates {
				dc.logger.Info(
					"Updated dimension",
					zap.String("dimensionUpdate", dimUpdate.String()),
				)
			}
		})))

	dc.requestSender.Send(req)

	return nil
}

func (dc *DimensionClient) makeDimURL(key, value string) (*url.URL, error) {
	url, err := dc.APIURL.Parse(fmt.Sprintf("/v2/dimension/%s/%s", url.PathEscape(key), url.PathEscape(value)))
	if err != nil {
		return nil, fmt.Errorf("could not construct dimension property PATCH URL with %s / %s: %v", key, value, err)
	}

	return url, nil
}

func (dc *DimensionClient) makePatchRequest(dim *DimensionUpdate) (*http.Request, error) {
	json, err := json.Marshal(map[string]interface{}{
		"customProperties": dim.Properties,
		"tags":             dim.TagsToAdd,
		"tagsToRemove":     dim.TagsToRemove,
	})
	if err != nil {
		return nil, err
	}

	url, err := dc.makeDimURL(dim.Name, dim.Value)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		"PATCH",
		strings.TrimRight(url.String(), "/")+"/_/sfxagent",
		bytes.NewReader(json))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-SF-TOKEN", dc.Token)

	return req, nil
}
