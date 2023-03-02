package command

import (
	"context"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestReplaceShellString(t *testing.T) {
	tests := map[string]struct {
		input     string
		nonEscape bool
		want      string
	}{
		"non-escape-1": {`abc${HOME}bb`, true, `abc\$\{HOME\}bb`},
		"non-escape-2": {`abc$HOME--`, true, `abc\$HOME--`},
		"non-escape-3": {`abc${HOME:-$(ls)}bb`, true, `abc\$\{HOME:-\$\(ls\)\}bb`},

		"escape-1": {`abc${HOME}bb`, false, `abc${HOME}bb`},
		"escape-2": {`abc$HOME--`, false, `abc$HOME--`},
		"escape-3": {`abc${HOME:-$(ls)}bb`, false, `abc${HOME:-\$\(ls\)\}bb`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := ReplaceShellString(tc.input, tc.nonEscape)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestNewShell(t *testing.T) {
	cmd := New(
		[]string{"sh", "-c", `echo --%s-- "--%s--" '--%s--' %s`},
		"$HOME/$abc--", "${HOME}/$abc--", "${HOME}/$abc--", "abc;rm -rf /",
	)
	if diff := cmp.Diff(cmd.Args, []string{
		"sh", "-c", `echo --$HOME/$abc---- --${HOME}/$abc---- --\$\{HOME\}/\$abc---- abc\;rm\ -rf\ /`,
	}); diff != "" {
		t.Fatal(diff)
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
	err := cmd.OnExit(func() {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	start := time.Now()
	b, err := cmd.Context(ctx).Output()
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
