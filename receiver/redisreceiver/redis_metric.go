package redisreceiver

import (
	"fmt"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// An intermediate data type that allows us to define at startup which metrics to
// convert (from the string-string map we get from redisSvc) and how to convert them.
type redisMetric struct {
	key               string
	name              string
	units             string
	desc              string
	labels            map[string]string
	labelDescriptions map[string]string
	mdType            metricspb.MetricDescriptor_Type
}

func buildSingleProtoMetric(
	redisMetric *redisMetric,
	strVal string,
	t time.Time,
) (*metricspb.Metric, error) {
	pt, err := parsePoint(redisMetric, strVal)
	if err != nil {
		return nil, err
	}
	pbMetric := newProtoMetric(redisMetric, pt, t)
	return pbMetric, nil
}

func parsePoint(redisMetric *redisMetric, strVal string) (*metricspb.Point, error) {
	switch redisMetric.mdType {
	case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
		return strToInt64Point(strVal)
	case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
		return strToDoublePoint(strVal)
	}
	// The error below should never happen because types are confined to getDefaultRedisMetrics().
	// If there's a change and it does, this error will cause a test failure in TestAllMetrics
	// which expects no errors/warnings, and in TestRedisRunnable which expects the number of input
	// metrics to equal the number of output metrics. If there's a change to the tests and an
	// unexpected type is found, the only effect will be that that one metric will be missing
	// from the output, in which case the error below is treated as a warning and is logged. Metrics
	// collection shouldn't be adversely affected.
	return nil, fmt.Errorf("unexpected point type %v", redisMetric.mdType)
}
