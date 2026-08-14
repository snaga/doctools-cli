package cli

import (
	"bytes"

	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// exitSignal captures an ExitWithError / ExitWithErrorWithHint invocation.
// The hooks panic with this value so that the command body stops at the same
// point os.Exit would have stopped it in production. Without that, a command
// keeps running past its error check and dereferences the nil result.
type exitSignal struct {
	err  error
	hint string
}

// resetCommandFlags walks the whole command tree and restores every flag to its
// declared default. Cobra binds flags to package level variables that outlive a
// single RootCmd.Execute(), so without this a flag set by one test case leaks
// into the next one.
//
// Slice flags need separate handling. Once pflag has marked a slice value as
// changed, a further Set appends instead of overwriting, so repeated runs
// accumulate: passing --columns 0 twice yields [0 0]. Replace empties the slice
// properly. Every slice flag in this CLI declares a nil default, so emptying is
// the correct reset for all of them.
func resetCommandFlags() {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}

	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(reset)
		cmd.PersistentFlags().VisitAll(reset)
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(RootCmd)
}

// runCLI executes a command with a clean flag state and reports whether the
// command aborted through util.ExitWithError / util.ExitWithErrorWithHint.
func runCLI(args ...string) (out string, cmdErr error, exit *exitSignal) {
	resetCommandFlags()

	buf := new(bytes.Buffer)

	oldWriter := util.OutputWriter
	oldHook := util.ExitWithErrorHook
	oldHintHook := util.ExitWithErrorWithHintHook

	util.OutputWriter = buf
	util.ExitWithErrorHook = func(err error) { panic(&exitSignal{err: err}) }
	util.ExitWithErrorWithHintHook = func(err error, hint string) { panic(&exitSignal{err: err, hint: hint}) }

	defer func() {
		util.OutputWriter = oldWriter
		util.ExitWithErrorHook = oldHook
		util.ExitWithErrorWithHintHook = oldHintHook
		out = buf.String()

		if r := recover(); r != nil {
			s, ok := r.(*exitSignal)
			if !ok {
				panic(r)
			}
			exit = s
		}
	}()

	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs(args)
	cmdErr = RootCmd.Execute()
	return
}
