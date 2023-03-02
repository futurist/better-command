//go:build !windows
// +build !windows

package command

import (
	"fmt"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// AsUser run command with osuser
func (c *Command) AsUser(osuser string) *Command {
	if runtime.GOOS == "windows" {
		c.LastError = fmt.Errorf("AsUesr: not support windows yet")
		return c
	}
	u, err := user.Lookup(osuser)
	if err != nil {
		c.LastError = fmt.Errorf("AsUesr: %w", err)
		return c
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		c.LastError = fmt.Errorf("AsUesr: %w", err)
		return c
	}
	c.Cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: uint32(uid),
	}
	hasHome := false
	envs := c.Cmd.Env
	for i, v := range envs {
		// fix user HOME env
		if strings.HasPrefix(v, "HOME=") {
			envs[i] = "HOME=" + u.HomeDir
			hasHome = true
		}
	}
	if !hasHome {
		envs = append(envs, "HOME="+u.HomeDir)
	}
	c.Cmd.Env = envs
	return c
}