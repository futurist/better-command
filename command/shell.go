// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/futurist/better-command/shlex"
)

const shellVars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"

var shellNormal = make(map[rune]bool, 0)

func init() {
	for _, v := range "#%+-.~/:=" + shellVars {
		shellNormal[v] = true
	}
}

func ReplaceShellString(s string, nonEscape bool) string {
	r := new(strings.Builder)
	inVar := 0
	for i, v := range s {
		next, next2 := "", ""
		if i+1 < len(s) {
			next = s[i+1 : i+2]
		}
		if i+2 < len(s) {
			next2 = s[i+2 : i+3]
		}
		if !nonEscape {
			if inVar == 1 && !strings.Contains(shellVars, string(v)) {
				inVar = 0
			}
			if inVar == 2 && v != '{' && !strings.Contains(shellVars, string(v)) {
				inVar = 0
				r.WriteRune(v)
				continue
			}
			if v == '$' && strings.Contains(shellVars, next) {
				inVar = 1
			}
			if v == '$' && next == "{" && strings.Contains(shellVars, next2) {
				inVar = 2
			}
			// $VAR || ${VAR}
			if inVar > 0 {
				r.WriteRune(v)
				continue
			}
			if !shellNormal[v] {
				r.WriteRune('\\')
			}
		}
		r.WriteRune(v)
	}
	return r.String()
}

// Command is embeded [exec.Cmd] struct, with some more state to use.
type Command struct {
	exec.Cmd
	// Pid is the pid of command after start
	Pid int
	// LastError is the last recorded error after chain
	LastError error
	// Ctx is the context of the command, can check Err() on OnExit to see if the context be canceled
	Ctx context.Context
	// Cancel the context of the command, command will be killed, and Ctx.Err() not nil
	Cancel  context.CancelFunc
	onstart []func(*Command)
	onexit  []func(*Command)
	mu      *sync.RWMutex
}

// sudo will return "sudo" command if non-root, or else ""
func sudo() []string {
	currentUser, _ := user.Current()
	if currentUser != nil {
		if currentUser.Uid == "0" {
			return nil
		}
		return []string{"sudo", "-E"}
	}
	return nil
}

// UseSudo to run command use `sudo` if not root, otherwise run normally
func (c *Command) UseSudo() *Command {
	s := sudo()
	if s != nil {
		c.Cmd.Args = append(s, c.Cmd.Args...)
	}
	return c
}

// Context can set command context that can cause the command be killed when canceled.
//
// The provided context is used to kill the process (by calling
// os.Process.Kill) if the context becomes done before the command
// completes on its own.
func (c *Command) Context(ctx context.Context) *Command {
	go func() {
		select {
		case <-ctx.Done():
			c.Cancel()
			c.Cancel = nil
		case <-c.Ctx.Done():
		}
		c.cleanup()
	}()
	return c
}

// Timeout run command with timeout, then kill the process.
func (c *Command) Timeout(timeout time.Duration) *Command {
	ctx, cancel := context.WithTimeout(c.Ctx, timeout)
	c.mu.Lock()
	c.onexit = append(c.onexit, func(c *Command) { cancel() })
	c.mu.Unlock()
	return c.Context(ctx)
}

// Env set command env to run
func (c *Command) Env(env []string) *Command {
	c.Cmd.Env = env
	return c
}

// Dir run command with PWD set to dir
func (c *Command) Dir(dir string) *Command {
	c.Cmd.Dir = dir
	return c
}

// Stdin set command stdin to f
func (c *Command) Stdin(f io.Reader) *Command {
	c.Cmd.Stdin = f
	return c
}

// Stdout set command stdout to f
func (c *Command) Stdout(f io.Writer) *Command {
	c.Cmd.Stdout = f
	return c
}

// Stderr set command stderr to f
func (c *Command) Stderr(f io.Writer) *Command {
	c.Cmd.Stderr = f
	return c
}

// Shell set command shell to shellName instead of 'sh', it must accept '-c' as second arg
func (c *Command) Shell(shellName string) *Command {
	c.Cmd.Args[0] = shellName
	return c
}

// OnStart set functions to run when command just started
func (c *Command) OnStart(f ...func(*Command)) *Command {
	c.mu.Lock()
	c.onstart = append(c.onstart, f...)
	c.mu.Unlock()
	return c
}

// OnExit set functions to run when command just exit,
// here can check the Ctx.Err() etc.
func (c *Command) OnExit(f ...func(*Command)) *Command {
	c.mu.Lock()
	c.onexit = append(c.onexit, f...)
	c.mu.Unlock()
	return c
}

// NewSh just like [New], but run []string{"sh", "-c", cmdString} by default
func NewSh(cmdString string, parts ...string) *Command {
	return New([]string{"sh", "-c", cmdString}, parts...)
}

// New return a Command instance to execute the named program with
// the given arguments, cmdArgs will be safely escaped, to avoid Remote Code Execution (RCE) attack
// or any form of Shell Injection, the escape will be denoted by below 2 forms:
//
//   - %s or "%s": will escape everything, except for shell variables like $ABC, or ${ABC}, any other variables form not accepted.
//   - '%s': will escape everything, shell variables also be escaped.
//
// Command returns the Cmd struct to execute the named program with
// the given arguments.
//
// It sets only the Path and Args in the returned structure.
//
// If name contains no path separators, Command uses LookPath to
// resolve name to a complete path if possible. Otherwise it uses name
// directly as Path.
//
// The returned Cmd's Args field is constructed from the command name
// followed by the elements of arg, so arg should not include the
// command name itself. For example, Command("echo", "hello").
// Args[0] is always name, not the possibly resolved Path.
//
// On Windows, processes receive the whole command line as a single string
// and do their own parsing. Command combines and quotes Args into a command
// line string with an algorithm compatible with applications using
// CommandLineToArgvW (which is the most common way). Notable exceptions are
// msiexec.exe and cmd.exe (and thus, all batch files), which have a different
// unquoting algorithm. In these or other similar cases, you can do the
// quoting yourself and provide the full command line in SysProcAttr.CmdLine,
// leaving Args empty.
func New(cmdArgs []string, parts ...string) *Command {
	for i2, v := range cmdArgs {
		c := make([]string, 0)
		l := shlex.NewTokenizer(strings.NewReader(v))
		i := 0
		for {
			if token, err := l.Next(); err != nil {
				break
			} else {
				s := token.String()
				for strings.Contains(s, "%s") {
					sanitized := ReplaceShellString(parts[i], token.IsNonEscape())
					s = strings.Replace(s, "%s", sanitized, 1)
					i++
				}
				c = append(c, s)
			}
		}
		cmdArgs[i2] = strings.Join(c, "")
	}

	// in go1.20 we should use context.WithCancelCause
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	if cmd == nil {
		cancel()
		return nil
	}

	c := &Command{Cmd: *cmd, Ctx: ctx, Cancel: cancel, mu: new(sync.RWMutex)}
	c.onexit = make([]func(*Command), 0)
	fn := c.initCmd(cmd)
	if fn != nil {
		c.onexit = append(c.onexit, fn)
	}
	return c
}

func (c *Command) cleanup() {
	c.mu.Lock()
	onexit := c.onexit
	c.onexit = nil
	c.mu.Unlock()
	for _, f := range onexit {
		f(c)
	}
	if c.Cancel != nil {
		c.Cancel()
	}
}

// resulting

// Run starts the specified command and waits for it to complete.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command starts but does not complete successfully, the error is of
// type *ExitError. Other error types may be returned for other situations.
//
// If the calling goroutine has locked the operating system thread
// with runtime.LockOSThread and modified any inheritable OS-level
// thread state (for example, Linux or Plan 9 name spaces), the new
// process will inherit the caller's thread state.
func (c *Command) Run() error {
	defer c.cleanup()
	if c.LastError != nil {
		return c.LastError
	}

	if err := c.Start(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.Process != nil {
		c.Pid = c.Process.Pid
	}
	onstart := c.onstart
	c.mu.Unlock()
	for _, v := range onstart {
		v(c)
	}
	return c.Wait()
}

// Output runs the command and returns its standard output.
// Any returned error will usually be of type *ExitError.
// If c.Stderr was nil, Output populates ExitError.Stderr.
func (c *Command) Output() ([]byte, error) {
	defer c.cleanup()
	if c.LastError != nil {
		return nil, c.LastError
	}

	if c.Cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	c.Cmd.Stdout = &stdout

	captureErr := c.Cmd.Stderr == nil
	if captureErr {
		c.Cmd.Stderr = &prefixSuffixSaver{N: 32 << 10}
	}

	err := c.Run()
	if err != nil && captureErr {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = c.Cmd.Stderr.(*prefixSuffixSaver).Bytes()
		}
	}
	return stdout.Bytes(), err
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
func (c *Command) CombinedOutput() ([]byte, error) {
	defer c.cleanup()
	if c.LastError != nil {
		return nil, c.LastError
	}

	if c.Cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Cmd.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Cmd.Stdout = &b
	c.Cmd.Stderr = &b
	err := c.Run()
	return b.Bytes(), err
}
