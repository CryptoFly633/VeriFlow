/*
Package cmd includes relayer commands
Copyright © 2020 Jack Zampolin <jack.zampolin@gmail.com>

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
package cmd

import (
	"fmt"
	"github.com/cosmos/relayer/v2/specy"
	specyconfig "github.com/cosmos/relayer/v2/specy/config"
	"strings"

	"github.com/spf13/cobra"
)

func executorCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "executor",
		Aliases: []string{"exe"},
		Short:   "Manage task executor",
	}

	cmd.AddCommand(
		executorCreateCmd(a),
	)

	return cmd
}

func executorCreateCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create [ias_report] [enclave_pk]",
		Aliases: []string{"st"},
		Short:   "Create the executor on specy chain",
		Args:    withUsage(cobra.MinimumNArgs(0)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s executor create 'iasreport' 'enclavepk'`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			var iasReport, enclavePK string
			if len(args) == 2 {
				// 优先取命令行中定义的值
				iasReport = args[0]
				enclavePK = args[1]
			} else {
				// 命令行没有指定则读取配置文件
				iasReport = specyconfig.Config.EngineInfo.IASReport
				enclavePK = specyconfig.Config.EngineInfo.EnclavePK
			}
			specy.CreateExecutor(iasReport, enclavePK)
			return nil
		},
	}
	return cmd
}
