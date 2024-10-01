package core

import "time"

type Clock struct {
	startTime float64
	elapsed   float64
}

func NewClock() *Clock {
	return &Clock{}
}

// Updates the provided clock. Should be called just before checking elapsed time.
// Has no effect on non-started clocks.
func (c *Clock) Update() {
	if c.startTime != 0 {
		c.elapsed = float64(time.Now().UnixNano()) - c.startTime
	}
}

// Starts the provided clock. Resets elapsed time.
func (c *Clock) Start() {
	c.startTime = float64(time.Now().UnixNano())
	c.elapsed = 0
}

// Stops the provided clock. Does not reset elapsed time.
func (c *Clock) Stop() {
	c.startTime = 0
}

func (c *Clock) Elapsed() float64 {
	return c.elapsed
}
