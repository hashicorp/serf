package agent

import (
	"fmt"
)

// FlagEventScripts is a type that implements the flag.Value interface
// for collecting multiple flags from the command-line.
type FlagEventScripts []EventScript

func (f *FlagEventScripts) String() string {
	return fmt.Sprintf("%#v", *f)
}

func (f *FlagEventScripts) Set(v string) error {
	scripts, err := ParseEventScript(v)
	if err != nil {
		return err
	}

	*f = append(*f, scripts...)
	return nil
}
