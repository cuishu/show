//go:build !linux

package shell

import "fmt"

func Shell() {
	fmt.Println("only support linux")
}
