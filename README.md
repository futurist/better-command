# better-command

Go better command run shell commands safely and handily.

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

### Chained style with handily functions

```go
import "github.com/futurist/better-command/command"

command.NewSh(`echo %s '%s'`, "logs: $HOME/$abc/logs", "logs: $HOME/$abc/logs")
    .Stdout(os.Stdout)
    .Stdin(os.Stdin)
    .Timeout(time.Second*10)
    .CombinedOutput()
```

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
