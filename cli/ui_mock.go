package cli

import (
	"bytes"
	"fmt"
)

// MockUi is a mock UI that is used for tests and is exported publicly for
// use in external tests if needed as well.
type MockUi struct {
	ErrorWriter  *bytes.Buffer
	OutputWriter *bytes.Buffer
}

func (u *MockUi) Error(message string) {
	fmt.Fprint(u.ErrorWriter, message)
	fmt.Fprint(u.ErrorWriter, "\n")
}

func (u *MockUi) Output(message string) {
	fmt.Fprint(u.OutputWriter, message)
	fmt.Fprint(u.OutputWriter, "\n")
}
