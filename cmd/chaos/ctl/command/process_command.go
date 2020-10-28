// Copyright 2020 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package command

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/chaos-mesh/chaos-daemon/pkg/core"
)

var (
	process string
)

func NewProcessAttackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "process <subcommand>",
		Short: "Process attack related commands",
	}

	cmd.AddCommand(
		NewProcessKillCommand(),
	)

	return cmd
}

func NewProcessKillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kill [options]",
		Short: "kill process, default signal 9",
		Run:   processKillCommandFunc,
	}

	cmd.Flags().StringVarP(&process, "process", "p", "", "The process name or the process ID")

	return cmd
}

func processKillCommandFunc(cmd *cobra.Command, args []string) {
	if len(process) == 0 {
		ExitWithError(ExitBadArgs, errors.New("process not provided"))
	}

	cli := mustClientFromCmd(cmd)

	resp, err := cli.CreateProcessAttack(&core.ProcessCommand{
		Process: process,
		Signal:  syscall.SIGKILL,
	})

	if err != nil {
		ExitWithError(ExitError, err)
	}

	fmt.Println(resp)
}