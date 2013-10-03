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

func (u *BasicUi) Output(message string) {
	log.Printf("[INFO] ui: %s", message)
	fmt.Fprint(u.Writer, message)
	fmt.Fprint(u.Writer, "\n")
}
