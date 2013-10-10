package agent

import (
	"io"
	"testing"
)

func TestEventWriter_impl(t *testing.T) {
	var _ io.Writer = new(EventWriter)
}
