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
	"context"
	"errors"
	"fmt"
	specyexecutor "github.com/cosmos/relayer/v2/specy/executor"
	"strconv"
	"strings"

	specyconfig "github.com/cosmos/relayer/v2/specy/config"

	"github.com/cosmos/relayer/v2/scheduler"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// startCmd represents the start command
func startCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"st"},
		Short:   "Start the scheduler on specy chain",
		Args:    withUsage(cobra.MinimumNArgs(0)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s start           # start all configured paths`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {

			homePath, err := cmd.Flags().GetString(flagHome)
			if err != nil {
				return err
			}
			a.homePath = homePath

			initialBlockHistory, err := cmd.Flags().GetUint64(flagInitialBlockHistory)
			if err != nil {
				return err
			}

			// init specy network environment
			initSpecyNetwork(cmd.Context())

			specyChain := getSpecyChain(a)

			if err := ensureKeyExist(specyChain); err != nil {
				return err
			}

			lstErrCh := scheduler.StartListener(
				cmd.Context(),
				a.log,
				specyChain,
				nil,
				initialBlockHistory,
			)

			// Block until the error channel sends a message.
			// The context being canceled will cause the relayer to stop,
			// so we don't want to separately monitor the ctx.Done channel,
			// because we would risk returning before the relayer cleans up.
			if err := <-lstErrCh; err != nil && !errors.Is(err, context.Canceled) {
				a.log.Warn(
					"Listener start error",
					zap.Error(err),
				)
				return err
			}
			return nil
		},
	}
	cmd = updateTimeFlags(a.viper, cmd)
	cmd = strategyFlag(a.viper, cmd)
	cmd = debugServerFlags(a.viper, cmd)
	cmd = processorFlag(a.viper, cmd)
	cmd = initBlockFlag(a.viper, cmd)
	cmd = flushIntervalFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

// GetStartOptions sets strategy specific fields.
func GetStartOptions(cmd *cobra.Command) (uint64, uint64, error) {
	maxTxSize, err := cmd.Flags().GetString(flagMaxTxSize)
	if err != nil {
		return 0, 0, err
	}

	txSize, err := strconv.ParseUint(maxTxSize, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	maxMsgLength, err := cmd.Flags().GetString(flagMaxMsgLength)
	if err != nil {
		return txSize * MB, 0, err
	}

	msgLen, err := strconv.ParseUint(maxMsgLength, 10, 64)
	if err != nil {
		return txSize * MB, 0, err
	}

	return txSize * MB, msgLen, nil
}

func initSpecyNetwork(ctx context.Context) {
	specyconfig.ReadSpecyConfigInfo()
	fmt.Printf("SpecyConfig: %+v \n", specyconfig.Config)
	specyexecutor.ConnectSpecyEngineWithHeartbeat(ctx)
}

func getSpecyChain(a *appState) *scheduler.Chain {
	fmt.Printf("getSpecyChain... chainName: %s \n", specyconfig.Config.ChainInfo.ChainId)
	return a.config.Chains[specyconfig.Config.ChainInfo.ChainId]
}
