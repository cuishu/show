//go:build linux

package shell

import (
	"fmt"

	"github.com/cuishu/shellgo"
)

type Prompt struct {
	n int
}

func (prompt *Prompt) String() string {
	prompt.n++
	return fmt.Sprintf("%d-> ", prompt.n)
}

func Shell() {
	shellgo.Run(shellgo.Config{
		UseSysCmd: true,
		Prompt:    &Prompt{},
	})
}
