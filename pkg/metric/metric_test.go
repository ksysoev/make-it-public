package metric

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricService_IncrementCounter(t *testing.T) {
	service := NewMetricService()

	// Test incrementing a counter with no tags
	service.IncrementCounter("test_counter_no_tags", 1, nil)
	counter := service.(*metricService).counters["test_counter_no_tags"]
	assert.NotNil(t, counter, "Counter should be created")
	assert.Equal(t, 1.0, testutil.ToFloat64(counter.With(nil)), "Counter value should be incremented by 1")

	// Test incrementing a counter with tags
	tags := map[string]string{"label1": "value1", "label2": "value2"}
	service.IncrementCounter("test_counter_with_tags", 3, tags)
	counterWithTags := service.(*metricService).counters["test_counter_with_tags"]
	assert.NotNil(t, counterWithTags, "Counter with tags should be created")
	assert.Equal(t, 3.0, testutil.ToFloat64(counterWithTags.With(tags)), "Counter value should be incremented by 3")

	// Test incrementing an existing counter
	service.IncrementCounter("test_counter_with_tags", 2, tags)
	assert.Equal(t, 5.0, testutil.ToFloat64(counterWithTags.With(tags)), "Counter value should be incremented to 5")
}

func TestMetricService_RecordDuration(t *testing.T) {
	service := NewMetricService()

	// Test recording duration with no tags
	start := time.Now()
	service.RecordDuration("test_duration_no_tags", nil, func() {
		time.Sleep(10 * time.Millisecond)
	})
	duration := service.(*metricService).durations["test_duration_no_tags"]
	assert.NotNil(t, duration, "Duration metric should be created")
	assert.GreaterOrEqual(t, testutil.ToFloat64(duration.Observer), float64(time.Since(start).Milliseconds()), "Duration should be recorded")

	// Test recording duration with tags
	tags := map[string]string{"label1": "value1", "label2": "value2"}
	service.RecordDuration("test_duration_with_tags", tags, func() {
		time.Sleep(20 * time.Millisecond)
	})
	durationWithTags := service.(*metricService).durations["test_duration_with_tags"]
	assert.NotNil(t, durationWithTags, "Duration metric with tags should be created")
	assert.GreaterOrEqual(t, testutil.ToFloat64(durationWithTags.Observer), float64(20), "Duration with tags should be recorded")
}

func TestMetricService_getKeys(t *testing.T) {
	service := NewMetricService()

	// Test with no tags
	tags := map[string]string{}
	keys := service.(*metricService).getKeys(tags)
	assert.Equal(t, 0, len(keys), "Keys should be empty for no tags")

	// Test with multiple tags
	tags = map[string]string{"label1": "value1", "label2": "value2"}
	keys = service.(*metricService).getKeys(tags)
	assert.ElementsMatch(t, []string{"label1", "label2"}, keys, "Keys should match the tag keys")
}
