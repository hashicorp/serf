package cli

import (
	"bytes"
	"fmt"
	"sync"
)

// MockUi is a mock UI that is used for tests and is exported publicly for
// use in external tests if needed as well.
type MockUi struct {
	ErrorWriter  *bytes.Buffer
	OutputWriter *bytes.Buffer

	once sync.Once
}

func (u *MockUi) Error(message string) {
	u.once.Do(u.init)

	fmt.Fprint(u.ErrorWriter, message)
	fmt.Fprint(u.ErrorWriter, "\n")
}

func (u *MockUi) Info(message string) {
	u.Output(message)
}

func (u *MockUi) Output(message string) {
	u.once.Do(u.init)

	fmt.Fprint(u.OutputWriter, message)
	fmt.Fprint(u.OutputWriter, "\n")
}

func (u *MockUi) init() {
	u.ErrorWriter = new(bytes.Buffer)
	u.OutputWriter = new(bytes.Buffer)
}
