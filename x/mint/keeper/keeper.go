package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/osmosis-labs/osmosis/x/mint/types"
	poolincentivestypes "github.com/osmosis-labs/osmosis/x/pool-incentives/types"
	"github.com/tendermint/tendermint/libs/log"
)

// Keeper of the mint store
type Keeper struct {
	cdc              codec.BinaryMarshaler
	storeKey         sdk.StoreKey
	paramSpace       paramtypes.Subspace
	accountKeeper    types.AccountKeeper
	bankKeeper       types.BankKeeper
	distrKeeper      types.DistrKeeper
	epochKeeper      types.EpochKeeper
	hooks            types.MintHooks
	feeCollectorName string
}

// NewKeeper creates a new mint Keeper instance
func NewKeeper(
	cdc codec.BinaryMarshaler, key sdk.StoreKey, paramSpace paramtypes.Subspace,
	ak types.AccountKeeper, bk types.BankKeeper, dk types.DistrKeeper, epochKeeper types.EpochKeeper,
	feeCollectorName string,
) Keeper {
	// ensure mint module account is set
	if addr := ak.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the mint module account has not been set")
	}

	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		cdc:              cdc,
		storeKey:         key,
		paramSpace:       paramSpace,
		accountKeeper:    ak,
		bankKeeper:       bk,
		distrKeeper:      dk,
		epochKeeper:      epochKeeper,
		feeCollectorName: feeCollectorName,
	}
}

// CreateDeveloperVestingModuleAccount creates the module account for developer vesting.
func (k Keeper) CreateDeveloperVestingModuleAccount(ctx sdk.Context, amount sdk.Coin) {
	moduleAcc := authtypes.NewEmptyModuleAccount(
		types.DeveloperVestingModuleAcctName, authtypes.Minter)
	k.accountKeeper.SetModuleAccount(ctx, moduleAcc)

	k.bankKeeper.MintCoins(ctx, types.DeveloperVestingModuleAcctName, sdk.NewCoins(amount))
}

// _____________________________________________________________________

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// Set the mint hooks
func (k *Keeper) SetHooks(h types.MintHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set mint hooks twice")
	}

	k.hooks = h

	return k
}

// GetLastHalvenEpochNum returns last halven epoch number
func (k Keeper) GetLastHalvenEpochNum(ctx sdk.Context) int64 {
	store := ctx.KVStore(k.storeKey)
	b := store.Get(types.LastHalvenEpochKey)
	if b == nil {
		return 0
	}

	return int64(sdk.BigEndianToUint64(b))
}

// SetLastHalvenEpochNum set last halven epoch number
func (k Keeper) SetLastHalvenEpochNum(ctx sdk.Context, epochNum int64) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.LastHalvenEpochKey, sdk.Uint64ToBigEndian(uint64(epochNum)))
}

// get the minter
func (k Keeper) GetMinter(ctx sdk.Context) (minter types.Minter) {
	store := ctx.KVStore(k.storeKey)
	b := store.Get(types.MinterKey)
	if b == nil {
		panic("stored minter should not have been nil")
	}

	k.cdc.MustUnmarshalBinaryBare(b, &minter)
	return
}

// set the minter
func (k Keeper) SetMinter(ctx sdk.Context, minter types.Minter) {
	store := ctx.KVStore(k.storeKey)
	b := k.cdc.MustMarshalBinaryBare(&minter)
	store.Set(types.MinterKey, b)
}

// _____________________________________________________________________

// GetParams returns the total set of minting parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return params
}

// SetParams sets the total set of minting parameters.
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramSpace.SetParamSet(ctx, &params)
}

// _____________________________________________________________________

// MintCoins implements an alias call to the underlying supply keeper's
// MintCoins to be used in BeginBlocker.
func (k Keeper) MintCoins(ctx sdk.Context, newCoins sdk.Coins) error {
	if newCoins.Empty() {
		// skip as no coins need to be minted
		return nil
	}

	return k.bankKeeper.MintCoins(ctx, types.ModuleName, newCoins)
}

// GetPoolAllocatableAsset gets the balance of the `MintedDenom` from fees and returns coins according to the `AllocationRatio`
func (k Keeper) GetProportions(ctx sdk.Context, fees sdk.Coins, ratio sdk.Dec) sdk.Coin {
	params := k.GetParams(ctx)
	amount := fees.AmountOf(params.MintDenom)
	return sdk.NewCoin(params.MintDenom, amount.ToDec().Mul(ratio).TruncateInt())
}

// DistributeMintedCoins implements distribution of minted coins from mint to external modules.
func (k Keeper) DistributeMintedCoins(ctx sdk.Context, mintedCoins sdk.Coins) error {
	params := k.GetParams(ctx)
	proportions := params.DistributionProportions

	// allocate staking incentives into fee collector account to be moved to on next begin blocker by staking module
	stakingIncentivesCoins := sdk.NewCoins(k.GetProportions(ctx, mintedCoins, proportions.Staking))
	err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, k.feeCollectorName, stakingIncentivesCoins)
	if err != nil {
		return err
	}

	// allocate pool allocation ratio to pool-incentives module account account
	poolIncentivesCoins := sdk.NewCoins(k.GetProportions(ctx, mintedCoins, proportions.PoolIncentives))
	err = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, poolincentivestypes.ModuleName, poolIncentivesCoins)
	if err != nil {
		return err
	}

	devRewardCoins := sdk.NewCoins(k.GetProportions(ctx, mintedCoins, proportions.DeveloperRewards))
	// This is supposed to come from the developer vesting module address, not the mint module address
	// we over-allocated to the mint module address earlier though, so we burn it right here.
	k.bankKeeper.BurnCoins(ctx, types.ModuleName, devRewardCoins)
	if len(params.WeightedDeveloperRewardsReceivers) == 0 {
		// fund community pool when rewards address is empty
		k.distrKeeper.FundCommunityPool(ctx, devRewardCoins, k.accountKeeper.GetModuleAddress(types.DeveloperVestingModuleAcctName))
	} else {
		// allocate developer rewards to addresses by weight
		for _, w := range params.WeightedDeveloperRewardsReceivers {
			devRewardPortionCoins := sdk.NewCoins(k.GetProportions(ctx, devRewardCoins, w.Weight))
			if w.Address == "" {
				k.distrKeeper.FundCommunityPool(ctx, devRewardPortionCoins,
					k.accountKeeper.GetModuleAddress(types.DeveloperVestingModuleAcctName))
			} else {
				devRewardsAddr, err := sdk.AccAddressFromBech32(w.Address)
				if err != nil {
					return err
				}
				// If recipient is vesting account, pay to account according to its vesting condition
				err = k.bankKeeper.SendCoinsFromModuleToAccountOriginalVesting(
					ctx, types.DeveloperVestingModuleAcctName, devRewardsAddr, devRewardPortionCoins)
				if err != nil {
					return err
				}
			}
		}
	}

	communityPoolCoins := sdk.NewCoins(k.GetProportions(ctx, mintedCoins, proportions.CommunityPool))
	k.distrKeeper.FundCommunityPool(ctx, communityPoolCoins, k.accountKeeper.GetModuleAddress(types.ModuleName))

	// call an hook after the minting and distribution of new coins
	k.hooks.AfterDistributeMintedCoins(ctx, mintedCoins)

	return err
}
