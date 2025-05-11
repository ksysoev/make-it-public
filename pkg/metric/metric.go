package metric

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MeasurementService interface {
	// IncrementCounter increments a counter metric by the specified value. It accepts metric name and its labels.
	IncrementCounter(metricName string, by uint, tags map[string]string)

	// RecordDuration records the elapsed duration of a function execution. It accepts metric name, its labels, and the function to be executed
	RecordDuration(metricName string, tags map[string]string, fn func())

	// GetMetricByName retrieves a metric if exists. It is specifically meant for testing purposes.
	GetMetricByName(metricName string) (prometheus.Collector, bool)
}

type metricService struct {
	counters  map[string]*prometheus.CounterVec
	durations map[string]*Duration
}

type Duration struct {
	Observer *prometheus.HistogramVec
	Timer    *prometheus.Timer
}

var (
	instance MeasurementService
	once     sync.Once
)

func GetMetricService() MeasurementService {
	once.Do(func() {
		instance = &metricService{
			counters:  make(map[string]*prometheus.CounterVec),
			durations: make(map[string]*Duration),
		}
	})

	return instance
}

func (m *metricService) IncrementCounter(metricName string, by uint, tags map[string]string) {
	metric := m.getOrCreateCounterVec(metricName, m.getKeys(tags))
	metric.With(tags).Add(float64(by))
}

func (m *metricService) RecordDuration(metricName string, tags map[string]string, fn func()) {
	metric := m.getOrCreateDuration(m.durations, tags, metricName)
	defer metric.Timer.ObserveDuration()
	fn()
}

func (m *metricService) getOrCreateCounterVec(metricName string, tagKeys []string) *prometheus.CounterVec {
	if counter, exists := m.counters[metricName]; exists {
		return counter
	}

	counter := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: metricName,
		Help: "A counter for " + metricName,
	}, tagKeys)

	m.counters[metricName] = counter

	return counter
}

func (m *metricService) getOrCreateDuration(durations map[string]*Duration, tags map[string]string, metricName string) *Duration {
	if duration, exists := durations[metricName]; exists {
		return duration
	}

	observer := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: metricName,
		Help: "A histogram for " + metricName,
	}, m.getKeys(tags))

	timer := prometheus.NewTimer(observer.With(tags))
	duration := &Duration{
		Observer: observer,
		Timer:    timer,
	}

	durations[metricName] = duration

	return duration
}

func (m *metricService) getKeys(tags map[string]string) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}

	return keys
}

func (m *metricService) GetMetricByName(metricName string) (prometheus.Collector, bool) {
	if counter, exists := m.counters[metricName]; exists {
		return counter, true
	}

	if duration, exists := m.durations[metricName]; exists {
		return duration.Observer, true
	}

	return nil, false
}
