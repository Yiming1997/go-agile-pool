package agilepool

import "time"

type Config struct {
	cleanPeriod       time.Duration
	taskQueueSize     int64
	workerNumCapacity int64
	workMode          WorkMode
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
