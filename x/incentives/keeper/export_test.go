package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) AddGaugeRefByKey(ctx sdk.Context, key []byte, guageID uint64) error {
	return k.addGaugeRefByKey(ctx, key, guageID)
}

func (k Keeper) DeleteGaugeRefByKey(ctx sdk.Context, key []byte, guageID uint64) {
	k.deleteGaugeRefByKey(ctx, key, guageID)
}

func (k Keeper) GetGaugeRefs(ctx sdk.Context, key []byte) []uint64 {
	return k.getGaugeRefs(ctx, key)
}

func (k Keeper) GetAllGaugeIDsByDenom(ctx sdk.Context, denom string) []uint64 {
	return k.getAllGaugeIDsByDenom(ctx, denom)
}
