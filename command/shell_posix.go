//go:build !windows
// +build !windows

package command

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func (c *Command) initCmd(cmd *exec.Cmd) func(*Command) {
	// Force-enable setpgid bit so that we can kill child processes when the
	// context is canceled.
	cmd.SysProcAttr.Setpgid = true
	killChild := func(c *Command) {
		c.mu.RLock()
		pid := c.Pid
		c.mu.RUnlock()
		if pid == 0 || c.Ctx.Err() == nil {
			return
		}
		// Kill by negative PID to kill the process group, which includes
		// the top-level process we spawned as well as any subprocesses
		// it spawned.
		err := syscall.Kill(-pid, syscall.SIGKILL)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kill:", err)
		}
	}
	return killChild
}

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
