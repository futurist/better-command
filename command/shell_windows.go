//go:build windows
// +build windows

package command

func (c *Command) initCmd(cmd *exec.Cmd) func(*Command) {
	return nil
}

// AsUser run command with osuser
func (c *Command) AsUser(osuser string) *Command {
	c.LastError = fmt.Errorf("AsUesr: not support windows yet")
	return c
}
