package tray

import "time"

func timeAfter2s() <-chan time.Time {
	return time.After(2 * time.Second)
}
