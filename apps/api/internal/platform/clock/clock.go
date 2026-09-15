// Package clock abstracts time so services can be tested deterministically.
package clock

import "time"

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

// Real uses the system clock.
type Real struct{}

// Now implements Clock.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fixed always returns the same instant; for tests.
type Fixed struct{ T time.Time }

// Now implements Clock.
func (f Fixed) Now() time.Time { return f.T }
