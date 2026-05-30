package agilepool

import "time"

type Config struct {
	cleanPeriod       time.Duration
	taskQueueSize     int64
	workerNumCapacity int64
	workMode          WorkMode
	idleContainerType IdleContainerType
}

type ConfigOption func(*Config)

func NewConfig(opts ...ConfigOption) *Config {
	config := &Config{
		cleanPeriod:       defaultCleanPeriod,
		taskQueueSize:     defaultTaskQueueSize,
		workerNumCapacity: defaultMaxWorkerNumCapacity,
		workMode:          defaultWorkMode,
		idleContainerType: defaultIdleContainerType,
	}
	for _, opt := range opts {
		opt(config)
	}
	return config
}

func WithCleanPeriod(duration time.Duration) ConfigOption {
	return func(c *Config) {
		if duration > 0 {
			c.cleanPeriod = duration
		}
	}
}

func WithTaskQueueSize(size int64) ConfigOption {
	return func(c *Config) {
		if size > 0 {
			c.taskQueueSize = size
		}
	}
}

func WithWorkerNumCapacity(capacity int64) ConfigOption {
	return func(c *Config) {
		if capacity > 0 {
			c.workerNumCapacity = capacity
		}
	}
}

func WithBlockMode(workMode WorkMode) ConfigOption {
	return func(c *Config) {
		c.workMode = workMode
	}
}

func WithIdleContainerType(containerType IdleContainerType) ConfigOption {
	return func(c *Config) {
		c.idleContainerType = containerType
	}
}
