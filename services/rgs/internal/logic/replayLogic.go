package logic

import (
	"context"

	"fastgame/internal/model"
	"fastgame/pkg/money"
	"fastgame/pkg/prng"
	"fastgame/pkg/xerr"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReplayLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReplayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReplayLogic {
	return &ReplayLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReplayLogic) GetReplay(req *types.ReplayReq) (*types.ReplayResp, error) {
	record, err := l.svcCtx.ReplayStore.FindByRoundID(l.ctx, req.RoundId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, xerr.ErrInvalidRequest
		}
		return nil, err
	}
	return buildReplayResp(record.ServerSeed, record.ClientSeed, record.Nonce, record.BetAmount, "default", record.RoundID, record.SequenceID)
}

func (l *ReplayLogic) ComputeReplay(req *types.ReplayComputeReq) (*types.ReplayResp, error) {
	rtpTier := req.RtpTier
	if rtpTier == "" {
		rtpTier = "default"
	}
	return buildReplayResp(req.ServerSeed, req.ClientSeed, req.Nonce, req.BetAmount, rtpTier, req.Nonce, 0)
}

func SceneToPayload(scene prng.ReplayScene, serverSeed, clientSeed, nonce string, betAmount money.Amount) types.ReplayPayload {
	path := make([]types.ReplayPoint, len(scene.FishPath))
	for i, p := range scene.FishPath {
		path[i] = types.ReplayPoint{X: p.X, Y: p.Y}
	}
	return types.ReplayPayload{
		Inputs: types.ReplayInputs{
			ServerSeed: serverSeed,
			ClientSeed: clientSeed,
			Nonce:      nonce,
			BetAmount:  betAmount.Minor(),
		},
		Scene: types.ReplayScene{
			Weather:        scene.Weather,
			FishSpecies:    scene.FishSpecies,
			FishPath:       path,
			BiteProp:       scene.BiteProp,
			CastDurationMs: scene.CastDurationMs,
			FishSpeed:      scene.FishSpeed,
		},
	}
}

func buildReplayResp(serverSeed, clientSeed, nonce string, betAmountMinor int64, rtpTier, roundID string, sequenceID uint64) (*types.ReplayResp, error) {
	betAmount := money.AmountFromMinor(betAmountMinor)
	engine := prng.NewEngine(rtpTier)
	defer engine.Release()
	scene, proof, err := engine.ComputeReplay(serverSeed, clientSeed, nonce, betAmount)
	if err != nil {
		return nil, err
	}

	return &types.ReplayResp{
		RoundId:      roundID,
		SequenceId:   sequenceID,
		Replay:       SceneToPayload(scene, serverSeed, clientSeed, nonce, betAmount),
		ProvablyFair: proofToTypes(proof),
		WinAmount:    scene.Outcome.WinAmount.Minor(),
		Multiplier:   scene.Outcome.Multiplier.Minor(),
		FishState:    scene.Outcome.FishState,
		AnimationKey: scene.Outcome.AnimationKey,
	}, nil
}

func proofToTypes(proof prng.FairProof) types.ProvablyFairProof {
	return types.ProvablyFairProof{
		ServerSeedHash: proof.ServerSeedHash,
		ServerSeed:     proof.ServerSeed,
		ClientSeed:     proof.ClientSeed,
		Nonce:          proof.Nonce,
		Roll:           prng.RollFloat(proof.Roll),
	}
}
