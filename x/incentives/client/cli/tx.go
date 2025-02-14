package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/osmosis-labs/osmosis/x/incentives/types"
	lockuptypes "github.com/osmosis-labs/osmosis/x/lockup/types"
	"github.com/spf13/cobra"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		NewCreateGaugeCmd(),
		NewAddToGaugeCmd(),
	)

	return cmd
}

// NewCreateGaugeCmd broadcast MsgCreateGauge
func NewCreateGaugeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-gauge [coins] [start_time] [num_epochs_paid_over] [flags]",
		Short: "create a gauge to distribute rewards to users",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			txf := tx.NewFactoryCLI(clientCtx, cmd.Flags()).WithTxConfig(clientCtx.TxConfig).WithAccountRetriever(clientCtx.AccountRetriever)
			coins, err := sdk.ParseCoinsNormalized(args[0])
			if err != nil {
				return err
			}

			timeUnix, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return err
			}
			startTime := time.Unix(timeUnix, 0)

			numEpochsPaidOver, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return err
			}

			queryTypeStr, err := cmd.Flags().GetString(FlagLockQueryType)
			if err != nil {
				return err
			}
			queryType, ok := lockuptypes.LockQueryType_value[queryTypeStr]
			if !ok {
				return errors.New("invalid lock query type, should be one of ByDuration or ByTime.")
			}
			denom, err := cmd.Flags().GetString(FlagDenom)
			if err != nil {
				return err
			}
			durationStr, err := cmd.Flags().GetString(FlagDuration)
			if err != nil {
				return err
			}
			duration, err := time.ParseDuration(durationStr)
			if err != nil {
				return err
			}
			timestamp, err := cmd.Flags().GetInt64(FlagTimestamp)
			if err != nil {
				return err
			}

			distributeTo := lockuptypes.QueryCondition{
				LockQueryType: lockuptypes.LockQueryType(queryType),
				Denom:         denom,
				Duration:      duration,
				Timestamp:     time.Unix(timestamp, 0),
			}

			// TODO: Confirm this is correct logic
			isPerpetual := false
			if numEpochsPaidOver == 0 {
				isPerpetual = true
			}

			msg := types.NewMsgCreateGauge(
				isPerpetual,
				clientCtx.GetFromAddress(),
				distributeTo,
				coins,
				startTime,
				numEpochsPaidOver,
			)

			return tx.GenerateOrBroadcastTxWithFactory(clientCtx, txf, msg)
		},
	}

	cmd.Flags().AddFlagSet(FlagSetCreateGauge())
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAddToGaugeCmd broadcast MsgAddToGauge
func NewAddToGaugeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-to-gauge [gauge_id] [rewards] [flags]",
		Short: "add coins to gauge to distribute more rewards to users",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			txf := tx.NewFactoryCLI(clientCtx, cmd.Flags()).WithTxConfig(clientCtx.TxConfig).WithAccountRetriever(clientCtx.AccountRetriever)

			gaugeId, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			rewards, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return err
			}

			msg := types.NewMsgAddToGauge(
				clientCtx.GetFromAddress(),
				gaugeId,
				rewards,
			)

			return tx.GenerateOrBroadcastTxWithFactory(clientCtx, txf, msg)
		},
	}

	cmd.Flags().AddFlagSet(FlagSetCreateGauge())
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
