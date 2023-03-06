# better-command

[![PkgGoDev](https://pkg.go.dev/badge/github.com/futurist/better-command)](https://pkg.go.dev/github.com/futurist/better-command)
[![Build Status](https://github.com/futurist/better-command/workflows/CI/badge.svg)](https://github.com/futurist/better-command/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/futurist/better-command)](https://goreportcard.com/report/github.com/futurist/better-command)
![Coverage](https://github.com/futurist/better-command/blob/badge/badge.svg?branch=badge)

Go better command run shell commands safely and handily.

More details please check [godoc](https://pkg.go.dev/github.com/futurist/better-command/command)

## Install

```sh
go get github.com/futurist/better-command
```

## Usage

### `New()` with `%s` as placeholder from args

```go
// below is true:
import "github.com/futurist/better-command/command"

reflect.DeepEqual(
    command.NewSh(`echo %s '%s'`, "logs: $HOME/$abc/logs", "logs: $HOME/$abc/logs").Args,
    []string{"sh", "-c", `echo logs\:\ $HOME/$abc/logs logs\:\ \$HOME/\$abc/logs `}
)
```

The argument for `'%s'` will be always safely escaped.

The argument for `%s` and `"%s"` will be always safely escaped except `$VAR` and `${VAR}`, thus you can use shell variables in side arguments.

The `New` and `NewSh` method argments just like `fmt.Printf`, the first arg is formatString, rest is format arguments, but with one exception: they can only accept `%s` as format placeholder. If you want use like `%v`, you can manually invoke `.toString()` method of the argument to pass as string.

### Chained style with handily functions

```go
import "github.com/futurist/better-command/command"

command.NewSh(`echo %s '%s'`, "logs: $HOME/$abc/logs", "logs: $HOME/$abc/logs")
    .Stdout(os.Stdout)
    .Stdin(os.Stdin)
    .Timeout(time.Second*10)
    .CombinedOutput()
```

There methods can be chained(in the middle):

- `UseSudo`
- `AsUser`
- `Timeout`
- `Context`
- `Env`
- `Dir`
- `Stdin`
- `Stdout`
- `Stderr`
- `Shell`
- `OnExit`

But below methods cannot be chained(finalize):

- `Run`
- `Output`
- `CombinedOutput`

### Default with context

```go
import "github.com/futurist/better-command/command"

cmd := command.New([]string{"bash", "-c", "sleep 10; echo ok"})
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(time.Millisecond * 100)
    cancel()
}()
cmd.Context(ctx).Run()
```

The command will be canceled in 100ms.

More details please see [godoc](https://pkg.go.dev/github.com/futurist/better-command/command):

[https://pkg.go.dev/github.com/futurist/better-command/command](https://pkg.go.dev/github.com/futurist/better-command/command)
