// Package command can be a better replacement of [exec/command] package,
// provide more safety and handy methods, chain style.
//
// # Safety
//
// The argument for '%s' will be always safely escaped.
//
// The argument for %s and "%s" will be always safely escaped except $VAR and ${VAR}, thus you can use shell variables in side arguments.
//
// Below is true and SAFE!!!:
//
// userID := httprequest.URL.Query().Get("userID") // maybe from a HACKER!!!
// fmt.Printf("userID: %v", userID) // userID: ;rm -rf /
// reflect.DeepEqual(
//     command.NewSh(`echo %s`, userID).Args,
//     []string{"sh", "-c", "echo \\;rm\\ -rf\\ /"}
// )
//
// The [New] and [NewSh] method will escape any invalid shell characters, to avoid Remote Code Execution (RCE) attack
// or any form of Shell Injection, the escape will be denoted by below 2 forms:
//   - %s or "%s": will escape everything, except for shell variables like $ABC, or ${ABC}, any other variables form not accepted.
//   - '%s': will escape everything, shell variables also be escaped.
//
// The [New]([]string, args...) and [NewSh](string, args...) method argments just like [fmt.Printf], the first arg is formatString, rest is format arguments, but with one exception: they can only accept %s as format placeholder. If you want use like %v, you can manually invoke [String()] method of the argument to pass as string.
//
// # Handy
//
// The package provider chain style invoking, like below:
//
//	command.NewSh(`echo %s '%s'`, "logs: $HOME/$abc/logs", "logs: $HOME/$abc/logs")
//		.Stdout(os.Stdout)
//		.Stdin(os.Stdin)
//		.Timeout(time.Second*10)
//		.CombinedOutput()
//
// There methods can be chained(in the middle):
//   - [command.UseSudo]
//   - [command.AsUser]
//   - [command.Timeout]
//   - [command.Context]
//   - [command.Env]
//   - [command.Dir]
//   - [command.Stdin]
//   - [command.Stdout]
//   - [command.Stderr]
//   - [command.Shell]
//   - [command.OnExit]
//
// But below methods cannot be chained(finalize):
//   - [command.Run]
//   - [command.Output]
//   - [command.CombinedOutput]
//
// For more information please checkout the godoc.
package command
