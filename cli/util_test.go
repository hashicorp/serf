package cli

import (
	"time"
)

func yield() {
	time.Sleep(5 * time.Millisecond)
}
