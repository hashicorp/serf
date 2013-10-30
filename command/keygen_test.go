package command

import (
	"github.com/hashicorp/serf/cli"
	"testing"
)

func TestKeygenCommand_implements(t *testing.T) {
	var _ cli.Command = &KeygenCommand{}
}
