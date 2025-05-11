package metric

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestGetMetricService(t *testing.T) {
	// Test singleton instance
	service1 := GetMetricService()
	service2 := GetMetricService()
	assert.Equal(t, service1, service2, "GetMetricService should return the same instance")
}

func TestMetricService_IncrementCounter(t *testing.T) {
	service := GetMetricService()

	// Test incrementing a counter with no tags
	service.IncrementCounter("test_counter_no_tags", 1, nil)
	counter, found := service.GetMetricByName("test_counter_no_tags")
	assert.True(t, found, "Counter should be found")
	assert.NotNil(t, counter, "Counter should be created")
	assert.Equal(t, 1.0, testutil.ToFloat64(counter), "Counter value should be incremented by 1")

	// Test incrementing a counter with tags
	tags := map[string]string{"label1": "value1", "label2": "value2"}
	service.IncrementCounter("test_counter_with_tags", 3, tags)
	counterWithTags, found := service.GetMetricByName("test_counter_with_tags")
	assert.True(t, found, "Counter with tags should be found")
	assert.NotNil(t, counterWithTags, "Counter with tags should be created")
	assert.Equal(t, 3.0, testutil.ToFloat64(counterWithTags), "Counter value should be incremented by 3")

	// Test incrementing an existing counter
	service.IncrementCounter("test_counter_with_tags", 2, tags)
	assert.Equal(t, 5.0, testutil.ToFloat64(counterWithTags), "Counter value should be incremented to 5")
}

func TestMetricService_RecordDuration(t *testing.T) {
	service := GetMetricService()

	// Test recording duration with no tags
	service.RecordDuration("test_duration_no_tags", nil, func() {
		time.Sleep(10 * time.Millisecond)
	})

	duration, found := service.GetMetricByName("test_duration_no_tags")

	assert.True(t, found, "Duration metric should be found")
	assert.NotNil(t, duration, "Duration metric should be created")
	assert.Equal(t, 1, testutil.CollectAndCount(duration), "Histogram should have one entry")

	// Test recording duration with tags
	tags := map[string]string{"label1": "value1", "label2": "value2"}
	service.RecordDuration("test_duration_with_tags", tags, func() {
		time.Sleep(20 * time.Millisecond)
	})

	durationWithTags, found := service.GetMetricByName("test_duration_no_tags")
	assert.True(t, found, "Duration metric with tags should be found")
	assert.NotNil(t, durationWithTags, "Duration metric with tags should be created")
	assert.Equal(t, 1, testutil.CollectAndCount(durationWithTags), "Histogram should have recorded at least one observation")
}

func TestMetricService_getKeys(t *testing.T) {
	service := &metricService{}

	// Test with no tags
	tags := map[string]string{}
	keys := service.getKeys(tags)
	assert.Equal(t, 0, len(keys), "Keys should be empty for no tags")

	// Test with multiple tags
	tags = map[string]string{"label1": "value1", "label2": "value2"}
	keys = service.getKeys(tags)
	assert.ElementsMatch(t, []string{"label1", "label2"}, keys, "Keys should match the tag keys")
}
