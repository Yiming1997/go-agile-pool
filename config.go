package agilepool

import "time"

type Config struct {
	cleanPeriod       time.Duration
	taskQueueSize     int64
	workerNumCapacity int64
	workMode          WorkMode
	idleContainerType IdleContainerType
	expiryDuration    time.Duration
}

func (c *Config) WithCleanPeriod(timeDuration time.Duration) *Config {
	c.cleanPeriod = timeDuration
	return c
}

func (c *Config) WithTaskQueueSize(taskQueueSize int64) *Config {
	c.taskQueueSize = taskQueueSize
	return c
}

func (c *Config) WithWorkerNumCapacity(workerNumCapacity int64) *Config {
	c.workerNumCapacity = workerNumCapacity
	return c
}

func (c *Config) WithBlockMode(workMode WorkMode) *Config {
	c.workMode = workMode
	return c
}

func (c *Config) WithIdleContainerType(containerType IdleContainerType) *Config {
	c.idleContainerType = containerType
	return c
}

// WithExpiryDuration sets how long an idle worker remains valid before
// being removed by the cleaner. Defaults to 1 second if not set.
func (c *Config) WithExpiryDuration(d time.Duration) *Config {
	c.expiryDuration = d
	return c
}
