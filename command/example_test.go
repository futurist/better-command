package command_test

import (
	"fmt"
	"strings"

	"github.com/futurist/better-command/command"
)

func ExampleNew() {
	out, err := command.New(
		[]string{"bash", "-c", "printf '%s' | awk '{print $0}'"}, "$(dangerous command) and $PASSWORD",
	).Output()
	// all the `$` will be escaped, so it's safe
	fmt.Println(string(out), err)
	// Output: $(dangerous command) and $PASSWORD
	//  <nil>
}

func ExampleNewSh() {
	out, err := command.NewSh(
		"printf '%s' | awk '{print $0}'", "$(dangerous command) and $PASSWORD",
	).Output()
	// all the `$` will be escaped, so it's safe
	fmt.Println(string(out), err)
	// Output: $(dangerous command) and $PASSWORD
	//  <nil>
}

func ExampleCommand_OnStart() {
	out, err := command.New(
		[]string{"bash", "-c", `echo "%s" | awk '{print '"$Var"' $0}'`}, "$(dangerous command) and normal $Var",
	).Env(
		[]string{"Var=123"},
	).OnStart(
		func(c *command.Command) {
			fmt.Printf("%#v", c.Cmd.Args)
		},
	).Output()
	// all the `$` will be escaped, so it's safe
	fmt.Println("--"+strings.TrimSpace(string(out))+"--", err)
	// Output: []string{"bash", "-c", "echo \\$\\(dangerous\\ command\\)\\ and\\ normal\\ $Var | awk '{print '$Var' $0}'"}--123$(dangerous command) and normal 123-- <nil>
}
