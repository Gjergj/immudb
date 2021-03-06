/*
Copyright 2019-2020 vChain, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package immuclient

import (
	"fmt"

	"github.com/codenotary/immudb/cmd/immuclient/audit"
	"github.com/codenotary/immudb/cmd/immuclient/cli"
	service "github.com/codenotary/immudb/cmd/immuclient/service/constants"
	"github.com/spf13/cobra"
)

func (cl *commandline) history(cmd *cobra.Command) {
	ccmd := &cobra.Command{
		Use:               "history key",
		Short:             "Fetch history for the item having the specified key",
		Aliases:           []string{"h"},
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.History(args)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.ExactArgs(1),
	}
	cmd.AddCommand(ccmd)
}

func (cl *commandline) status(cmd *cobra.Command) {
	ccmd := &cobra.Command{
		Use:               "status",
		Short:             "Ping to check if server connection is alive",
		Aliases:           []string{"p"},
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.HealthCheck(args)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(ccmd)
}

func (cl *commandline) auditmode(cmd *cobra.Command) {
	ccmd := &cobra.Command{
		Use:               "audit-mode command",
		Short:             "Starts immuclient as daemon in auditor mode. Run 'immuclient audit-mode help' or use -h flag for details",
		Aliases:           []string{"audit-mode"},
		Example:           service.UsageExamples,
		PersistentPostRun: cl.disconnect,
		ValidArgs:         []string{"help", "start", "install", "uninstall", "restart", "stop", "status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := audit.Init(args); err != nil {
				cl.quit(err)
			}
			return nil
		},
		Args: cobra.ExactArgs(1),
	}
	cmd.AddCommand(ccmd)
}

// #TODO will be new root.
func (cl *commandline) interactiveCli(cmd *cobra.Command) {
	ccmd := &cobra.Command{
		Use:     "it",
		Short:   "Starts immuclient in CLI mode. Use 'help' or -h flag on the shell for details",
		Aliases: []string{"cli-mode"},
		Example: cli.Init(cl.immucl).HelpMessage(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.Init(cl.immucl).Run()
			return nil
		},
	}
	cmd.AddCommand(ccmd)
}

func (cl *commandline) user(cmd *cobra.Command) {
	ccmd := &cobra.Command{
		Use:               "user command",
		Short:             "Issue all user commands",
		Aliases:           []string{"u"},
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
	}
	userListCmd := &cobra.Command{
		Use:               "list",
		Short:             "List all users",
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.UserList(args)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.MaximumNArgs(0),
	}
	userChangePassword := &cobra.Command{
		Use:               "changepassword",
		Short:             "Change user password. changepassword username",
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.ChangeUserPassword(args)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.MaximumNArgs(1),
	}

	userCreate := &cobra.Command{
		Use:               "create",
		Short:             "Create a new user",
		Long:              "Create a new user. user create username",
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.UserCreate(args)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.MaximumNArgs(4),
	}
	userActivate := &cobra.Command{
		Use:               "activate",
		Short:             "Activate a user",
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.SetActiveUser(args, true)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.MaximumNArgs(4),
	}
	userDeactivate := &cobra.Command{
		Use:               "deactivate",
		Short:             "Deactivate a user",
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.SetActiveUser(args, false)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.MaximumNArgs(4),
	}
	userPermission := &cobra.Command{
		Use:               "permission",
		Short:             "Set user permission",
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.SetUserPermission(args)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.MaximumNArgs(4),
	}

	ccmd.AddCommand(userListCmd)
	ccmd.AddCommand(userChangePassword)
	ccmd.AddCommand(userCreate)
	ccmd.AddCommand(userActivate)
	ccmd.AddCommand(userDeactivate)
	ccmd.AddCommand(userPermission)
	cmd.AddCommand(ccmd)
}

func (cl *commandline) use(cmd *cobra.Command) {
	ccmd := &cobra.Command{
		Use:               "use command",
		Short:             "Select database",
		Example:           "use {database_name}",
		PersistentPreRunE: cl.connect,
		PersistentPostRun: cl.disconnect,
		ValidArgs:         []string{"databasename"},
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cl.immucl.UseDatabase(args)
			if err != nil {
				cl.quit(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), resp+"\n")
			return nil
		},
		Args: cobra.MaximumNArgs(2),
	}
	cmd.AddCommand(ccmd)
}
