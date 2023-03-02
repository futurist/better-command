# go-shell

Go shell run shell commands safely and handily.

## Install

```sh
go get github.com/futurist/better-command
```

## Usage

### `New()` with `%s` as placeholder from args

```go
// below is true:
reflect.DeepEqual(
    shell.NewSh(`echo %s '%s'`, "logs: $HOME/$abc/logs", "logs: $HOME/$abc/logs").Args,
    []string{"sh", "-c", `echo logs\:\ $HOME/$abc/logs logs\:\ \$HOME/\$abc/logs `}
)
```

The argument for `'%s'` will be always safely escaped.

The argument for `%s` and `"%s"` will be always safely escaped except `$VAR` and `${VAR}`, thus you can use shell variables in side arguments.

### Chained style with handily functions

```go
shell.NewSh(`echo %s '%s'`, "logs: $HOME/$abc/logs", "logs: $HOME/$abc/logs")
    .Stdout(os.Stdout)
    .Stdin(os.Stdin)
    .Timeout(time.Second*10)
    .CombinedOutput()
```

### Default with context

```go
shell.New(``)
```
