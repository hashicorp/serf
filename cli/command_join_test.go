package cli

import (
	"testing"
)

func TestJoinCommand_implements(t *testing.T) {
	var _ Command = &JoinCommand{}
}
