package cli

import (
	"fmt"
	"io"
	"log"
)

// The Ui interface handles all communication to/from the console. This
// sort of interface allows us to strictly control how output is formatted.
type Ui interface {
	// Output is called for normal human output.
	Output(string)

	// Info is called for information related to the previous output.
	// In general this may be the exact same as Output, but this gives
	// Ui implementors some flexibility with output formats.
	Info(string)

	// Error is used for error messages.
	Error(string)
}

// BasicUi is an implementation of Ui that just outputs to the given
// writer.
type BasicUi struct {
	Writer io.Writer
}

func (u *BasicUi) Error(message string) {
	log.Printf("[INFO] ui: %s", message)
	fmt.Fprint(u.Writer, message)
	fmt.Fprint(u.Writer, "\n")
}

func (u *BasicUi) Info(message string) {
	u.Output(message)
}

func (u *BasicUi) Output(message string) {
	log.Printf("[INFO] ui: %s", message)
	fmt.Fprint(u.Writer, message)
	fmt.Fprint(u.Writer, "\n")
}
