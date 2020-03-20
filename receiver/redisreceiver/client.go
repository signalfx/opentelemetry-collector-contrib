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

package redisreceiver

import (
	"github.com/go-redis/redis/v7"
)

// Interface for a Redis client. Implementation can be fake or real.
type client interface {
	// retrieves a string of key/value pairs of redis metadata
	retrieveInfo() (string, error)
	// line delimiter
	// redis lines are delimited by \r\n, files (for testing) by \n
	delimiter() string
}

// Wraps a real redis client, implements client interface.
type redisClient struct {
	client *redis.Client
}

var _ client = (*redisClient)(nil)

func newRedisClient(options *redis.Options) client {
	return &redisClient{
		client: redis.NewClient(options),
	}
}

func (c *redisClient) delimiter() string {
	return "\r\n"
}

func (c *redisClient) retrieveInfo() (string, error) {
	return c.client.Info().Result()
}
