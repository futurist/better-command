package command

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/futurist/better-command/shlex"
	"github.com/google/go-cmp/cmp"
)

func TestReplaceShellString(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"space":     {`echo `, `echo\ `},
		"comment":   {`echo #test`, `echo\ #test`},
		"comment-1": {`echo '##'# test #`, `echo\ '##'# test #`},
		"comment-2": {`echo 
		#test`,
			`echo\ 
		#test`},

		"non-escape-1": {`'abc${HOME}bb'`, `'abc${HOME}bb'`},
		"non-escape-2": {`'abc$HOME--'`, `'abc$HOME--'`},
		"non-escape-3": {`'abc${HOME:-$(ls)}bb'`, `'abc${HOME:-$(ls)}bb'`},

		"escape-1": {`abc${HOME}bb`, `abc${HOME}bb`},
		"escape-2": {`abc$HOME--`, `abc$HOME--`},
		"escape-3": {`abc${HOME:-$(ls)}bb`, `abc\${HOME:-\$\(ls\)\}bb`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			l := shlex.NewTokenizer(strings.NewReader(tc.input))
			s := make([]string, 0)
			for {
				if token, err := l.Next(); err != nil {
					break
				} else {
					s = append(s, ReplaceShellString(token.String(), token))
				}
			}
			if diff := cmp.Diff(strings.Join(s, ""), tc.want); diff != "" {
				t.Fatal(diff, strings.Join(s, ""))
			}
		})
	}
}

func TestNewBash(t *testing.T) {
	cmd := NewBash("echo")
	if cmd.Args[0] != "bash" {
		t.Fatal("NewBash should have bash on args[0]")
	}
}

func TestNewShell(t *testing.T) {
	cmd := New(
		[]string{"sh", "-c", `echo --%s-- "--%s--" '--%s--' %s`},
		"$HOME/$abc--", "${HOME}/$abc--", "${HOME}/$abc--", "abc;rm -rf /",
	)
	if diff := cmp.Diff(cmd.Args, []string{
		"sh", "-c", `echo --$HOME/$abc---- --${HOME}\/$abc\-\--- '--${HOME}/$abc----' abc\;rm\ -rf\ /`,
	}); diff != "" {
		t.Fatal(diff, cmd.Args)
	}
}

func TestShellRun(t *testing.T) {
	name := "testrun-" + strconv.Itoa(rand.Int())
	cmd := New(
		[]string{"sh", "-c", `touch /tmp/%s`},
		name,
	)
	err := cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Open("/tmp/" + name)
	defer os.Remove("/tmp/" + name)
	if err != nil {
		t.Fatal(err)
	}
}

func TestShellRun2(t *testing.T) {
	name := "testrun-" + strconv.Itoa(rand.Int())
	cmd := NewSh(`touch /tmp/%s`, name)
	err := cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Open("/tmp/" + name)
	defer os.Remove("/tmp/" + name)
	if err != nil {
		t.Fatal(err)
	}
}

func TestShellOutput(t *testing.T) {
	cmd := NewSh(`printf abc; printf def 1>&2; exit 1`)
	b, err := cmd.Output()
	if err == nil {
		t.Fatal("error should not be nil")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Fatal("ExitCode should be 1")
	}
	if string(b) != "abc" {
		t.Fatal("stdout should be: abc")
	}
	hasError := false
	if ee, ok := err.(*exec.ExitError); ok {
		hasError = true
		if string(ee.Stderr) != "def" {
			t.Fatal("stderr should be: def")
		}
	}
	if hasError == false {
		t.Fatal("hasError must be true")
	}
}

func TestShellCombinedOutput(t *testing.T) {
	cmd := NewSh(`printf abc; printf def 1>&2`)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "abcdef" {
		t.Fatal("stdout should be: abcdef")
	}
}

func TestShellUseSudo(t *testing.T) {
	cmd := NewSh(`whoami`).UseSudo()
	b, err := cmd.Output()
	fmt.Println(string(b), err)
}

func TestShellEnv(t *testing.T) {
	cmd := NewSh(`printf $ABC`).Env([]string{"ABC=1"})
	b, _ := cmd.Output()
	if string(b) != "1" {
		t.Fatal("env should be 1", string(b))
	}
}

func TestShellDir(t *testing.T) {
	tmp, _ := os.Getwd()
	cmd := NewSh(`pwd`).Dir(tmp)
	b, _ := cmd.Output()

	out := path.Clean(strings.TrimSpace(string(b)))
	want := path.Clean(strings.TrimSpace(tmp))
	if out != want {
		t.Fatal("dir should be "+tmp, string(b))
	}
}

func TestShellStdin(t *testing.T) {
	buf := new(bytes.Buffer)
	buf.WriteString("abc")
	cmd := NewSh(`read a; printf $a`).Stdin(buf)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "abc" {
		t.Fatal("stdin should be abc")
	}
}

func TestShellStdout(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewSh(`printf abc`).Stdout(buf)
	_, err := cmd.Output()
	if err == nil {
		t.Fatal("should show error")
	}
	cmd = NewSh(`printf abc`).Stdout(buf)
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "abc" {
		t.Fatal("output should be abc")
	}
}

func TestShellStderr(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewSh(`printf abc`).Stderr(buf)
	_, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("should show error")
	}
	cmd = NewSh(`printf abc 1>&2`).Stderr(buf)
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "abc" {
		t.Fatal("outerr should be abc", buf.String())
	}
}

func TestShellBash(t *testing.T) {
	cmd := NewSh(`printf $0`)
	b, err := cmd.Shell("bash").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) != "bash" {
		t.Fatal("change shell to bash failed")
	}
}

func TestShellCleanup(t *testing.T) {
	name := "testrun-" + strconv.Itoa(rand.Int())
	file := path.Join("/tmp", name)
	cmd := NewSh(`touch /tmp/%s`, name)
	err := cmd.OnExit(func(*Command) {
		if cmd.Ctx.Err() != nil {
			t.Fatal("context should be nil")
		}
		os.Remove(file)
	}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatal("cleanup failed")
	}
}

func TestShellContext(t *testing.T) {
	cmd := NewSh(`sleep 1 ; printf ok`)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Millisecond * 100)
		cancel()
	}()
	start := time.Now()
	b, err := cmd.Context(ctx).Output()
	if time.Since(start) > time.Millisecond*200 {
		t.Fatal("should be killed")
	}
	if err == nil {
		t.Fatal("should error when canceled")
	}
	if err.Error() != "signal: killed" {
		t.Fatal("should signal: killed", err)
	}
	if cmd.Ctx.Err().Error() != "context canceled" {
		t.Fatal("should error with: context canceled")
	}
	if strings.TrimSpace(string(b)) == "ok" {
		t.Fatal("context failed")
	}
}

func TestShellTimeout(t *testing.T) {
	cmd := NewSh(`sleep 1; printf ok`)
	start := time.Now()
	b, err := cmd.Timeout(time.Millisecond * 100).Output()
	if time.Since(start) > time.Millisecond*200 {
		t.Fatal("should be killed")
	}
	if err == nil {
		t.Fatal("should error when canceled")
	}
	if err.Error() != "signal: killed" {
		t.Fatal("should signal: killed")
	}
	if cmd.Ctx.Err().Error() != "context canceled" {
		t.Fatal("should error with: context canceled")
	}
	if strings.TrimSpace(string(b)) == "ok" {
		t.Fatal("context failed")
	}
}
