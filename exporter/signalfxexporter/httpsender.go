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

package signalfxexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/open-telemetry/opentelemetry-collector/component/componenterror"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumererror"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

// httpSender sends the data to the SignalFx backend.
type httpSender struct {
	ingestURL *url.URL
	apiURL    *url.URL
	headers   map[string]string
	client    *http.Client
	logger    *zap.Logger
	zippers   sync.Pool
}

func (s *httpSender) pushMetricsData(
	ctx context.Context,
	md consumerdata.MetricsData,
) (droppedTimeSeries int, err error) {

	sfxDataPoints, numDroppedTimeseries, err := metricDataToSingalFxV2(s.logger, md)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	body, compressed, err := s.encodeBody(sfxDataPoints)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	req, err := http.NewRequest("POST", s.ingestURL.String(), body)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), consumererror.Permanent(err)
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return exporterhelper.NumTimeSeries(md), err
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	// SignalFx accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf(
			"HTTP %d %q",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		return exporterhelper.NumTimeSeries(md), err
	}

	return numDroppedTimeseries, nil
}

func buildHeaders(config *Config) (map[string]string, error) {
	headers := map[string]string{
		"Connection":   "keep-alive",
		"Content-Type": "application/x-protobuf",
		"User-Agent":   "OpenTelemetry-Collector SignalFx Exporter/v0.0.1",
	}

	if config.AccessToken != "" {
		headers["X-Sf-Token"] = config.AccessToken
	}

	// Add any custom headers from the config. They will override the pre-defined
	// ones above in case of conflict, but, not the content encoding one since
	// the latter one is defined according to the payload.
	for k, v := range config.Headers {
		headers[k] = v
	}

	return headers, nil
}

func (s *httpSender) encodeBody(dps []*sfxpb.DataPoint) (bodyReader io.Reader, compressed bool, err error) {
	msg := &sfxpb.DataPointUploadMessage{
		Datapoints: dps,
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		return nil, false, err
	}
	return s.getReader(body)
}

// avoid attempting to compress things that fit into a single ethernet frame
func (s *httpSender) getReader(b []byte) (io.Reader, bool, error) {
	var err error
	if len(b) > 1500 {
		buf := new(bytes.Buffer)
		w := s.zippers.Get().(*gzip.Writer)
		defer s.zippers.Put(w)
		w.Reset(buf)
		_, err = w.Write(b)
		if err == nil {
			err = w.Close()
			if err == nil {
				return buf, true, nil
			}
		}
	}
	return bytes.NewReader(b), false, err
}

func (s *httpSender) pushKubernetesMetadata(metadata map[string]*collection.KubernetesMetadataUpdate) error {
	var errs []error
	for _, m := range metadata {
		if err := s.pushKubernetesMetadataHelper(*m); err != nil {
			errs = append(errs, err)
		}
	}

	return componenterror.CombineErrors(errs)
}

func (s *httpSender) pushKubernetesMetadataHelper(metadata collection.KubernetesMetadataUpdate) error {
	dimensionUpdate := getDimensionUpdateFromMetadata(metadata)

	json, err := json.Marshal(map[string]interface{}{
		"customProperties": dimensionUpdate.properties,
		"tags":             dimensionUpdate.tagsToAdd,
		"tagsToRemove":     dimensionUpdate.tagsToRemove,
	})
	if err != nil {
		return err
	}

	url, err := s.makeDimURL(dimensionUpdate.dimensionKey, dimensionUpdate.dimensionValue)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", url.String(), bytes.NewReader(json))
	if err != nil {
		return err
	}

	s.headers["Content-Type"] = "application/json"
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println(body)

	return err
}

func (s *httpSender) makeDimURL(key, value string) (*url.URL, error) {
	url, err := s.apiURL.Parse(fmt.Sprintf("/v2/dimension/%s/%s/_/sfxagent", url.PathEscape(key), url.PathEscape(value)))
	if err != nil {
		return nil, fmt.Errorf("could not construct dimension property PUT URL with %s / %s: %v", key, value, err)
	}

	return url, nil
}