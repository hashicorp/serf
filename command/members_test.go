package command

import (
	"github.com/hashicorp/serf/cli"
	"testing"
)

func TestMembersCommand_implements(t *testing.T) {
	var _ cli.Command = &MembersCommand{}
}
