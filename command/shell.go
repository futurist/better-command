package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
		}
		if !shellNormal[v] {
			r.WriteRune('\\')
		}
		r.WriteRune(v)
	}
	return r.String()
}

type Command struct {
	exec.Cmd
	Pid       int
	LastError error
	Ctx       context.Context
	cancel    context.CancelFunc
	onexit    []func()
	mu        *sync.Mutex
}

func (c *Command) Context(ctx context.Context) *Command {
	go func() {
		select {
		case <-ctx.Done():
		case <-c.Ctx.Done():
		}
		c.cancel()
		c.cleanup()
	}()
	return c
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

func (c *Command) UseSudo() *Command {
	s := sudo()
	if s != nil {
		c.Cmd.Args = append(s, c.Cmd.Args...)
	}
	return c
}

func (c *Command) Timeout(timeout time.Duration) *Command {
	ctx, cancel := context.WithTimeout(c.Ctx, timeout)
	c.onexit = append(c.onexit, cancel)
	return c.Context(ctx)
}

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

func (c *Command) Env(env []string) *Command {
	c.Cmd.Env = env
	return c
}

func (c *Command) Dir(dir string) *Command {
	c.Cmd.Dir = dir
	return c
}

func (c *Command) Stdin(f io.Reader) *Command {
	c.Cmd.Stdin = f
	return c
}

func (c *Command) Stdout(f io.Writer) *Command {
	c.Cmd.Stdout = f
	return c
}

func (c *Command) Stderr(f io.Writer) *Command {
	c.Cmd.Stderr = f
	return c
}

func (c *Command) Shell(shellName string) *Command {
	c.Cmd.Args[0] = shellName
	return c
}

func (c *Command) OnExit(f ...func()) *Command {
	c.onexit = append(c.onexit, f...)
	return c
}

func NewSh(cmdString string, parts ...string) *Command {
	return New([]string{"sh", "-c", cmdString}, parts...)
}

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
		cmdArgs[i2] = strings.Join(c, " ")
	}

	// in go1.20 we should use context.WithCancelCause
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	if cmd == nil {
		cancel()
		return nil
	}

	// Force-enable setpgid bit so that we can kill child processes when the
	// context is canceled.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	c := &Command{Cmd: *cmd, Ctx: ctx, cancel: cancel, mu: new(sync.Mutex)}
	killChild := func() {
		if c.Pid == 0 || ctx.Err() == nil {
			return
		}
		// Kill by negative PID to kill the process group, which includes
		// the top-level process we spawned as well as any subprocesses
		// it spawned.
		err := syscall.Kill(-c.Pid, syscall.SIGKILL)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kill:", err)
		}
	}
	c.onexit = []func(){killChild}
	return c
}

func (c *Command) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	onexit := c.onexit
	c.onexit = nil
	for _, f := range onexit {
		f()
	}
	c.cancel()
}

// resulting

func (c *Command) Run() error {
	defer c.cleanup()
	if c.LastError != nil {
		return c.LastError
	}

	if err := c.Start(); err != nil {
		return err
	}
	if c.Process != nil {
		c.Pid = c.Process.Pid
	}
	return c.Wait()
}

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
