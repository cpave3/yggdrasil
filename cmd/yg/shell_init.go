package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const shellIntegration = `yg() {
  if [ "${1-}" = "cd" ]; then
    shift
    local yg_path
    yg_path="$(command yg cd "$@")" || return
    builtin cd -- "$yg_path"
  else
    command yg "$@"
  fi
}
`

func shellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init",
		Short: "Print Bash/Zsh integration",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprint(cmd.OutOrStdout(), shellIntegration)
		},
	}
}
