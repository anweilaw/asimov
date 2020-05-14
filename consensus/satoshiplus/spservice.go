// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package satoshiplus

import (
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/ainterface"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/consensus/params"
	"sync"
	"time"
)

// Prove of SatoshiPlus.
// using round robin algorithm to generate blocks with orders in the list of authority peers.
// this consensus depends on bitcoin's miner.
// please refer to the white paper of asimov.
type SPService struct {
	sync.Mutex
	wg         sync.WaitGroup
	existCh    chan interface{}
	context    params.Context
	blockTimer *time.Timer
	roundTimer *time.Timer

	config *params.Config

	chainTipChan chan ainterface.BlockNode
}

// Create SatoshiPlus Service
func NewSatoshiPlusService(config *params.Config) (*SPService, error) {
	if config == nil {
		return nil, errors.New("config can't be nil")
	}
	if config.Chain == nil {
		return nil, errors.New("config.chain can't be nil")
	}
	service := &SPService{
		config:       config,
		chainTipChan: make(chan ainterface.BlockNode),
	}

	config.Chain.Subscribe(service.handleBlockchainNotification)
	return service, nil
}

// Start SatoshiPlus Service, it create a goroutine and check mining on blockTimer
func (s *SPService) Start() error {
	s.Lock()
	defer s.Unlock()
	if s.existCh != nil {
		return errors.New("satoshiplus consensus is already started")
	}
	if s.config.Account == nil {
		log.Warn("satoshiplus service exit when account is nil")
		return nil
	}
	log.Info("satoshiplus consensus start")

	// current block maybe do not at the best block height:
	s.blockTimer = time.NewTimer(common.DefaultBlockInterval * 1000000)
	s.roundTimer = time.NewTimer(common.DefaultBlockInterval * 1000000)
	if err := s.initializeConsensus(); err != nil {
		log.Errorf("Start satoshi service failed:", err)
		s.blockTimer.Stop()
		s.blockTimer = nil
		s.roundTimer.Stop()
		s.roundTimer = nil
		return err
	}

	s.existCh = make(chan interface{})
	s.wg.Add(1)
	go func() {
		existCh := s.existCh
	mainloop:
		for {
			select {
			case <-s.blockTimer.C:
				block, genErr := s.genBlock()
				if genErr == nil && block != nil {
					_, processErr := s.config.ProcessBlock(block, common.BFNone)
					if processErr != nil {
						// Anything other than a rule violation is an unexpected error,
						// so log that error as an internal error.
						if _, ok := processErr.(blockchain.RuleError); !ok {
							log.Errorf("Unexpected error while processing "+
								"block submitted via satoshi miner: %v", processErr)
						}
						log.Errorf("satoshiplus gen block submit reject, height=%d, %v", block.Height(), processErr)
					} else {
						log.Infof("satoshiplus gen block submit accept, height=%d, hash=%s, sigNum=%v, txNum=%d",
							block.Height(), block.Hash().String(),
							len(block.MsgBlock().PreBlockSigs), len(block.MsgBlock().Transactions))
					}
				}
			case <- s.roundTimer.C:

			case <-existCh:
				break mainloop
			}
		}
		s.blockTimer.Stop()
	cleanup:
		for {
			select {
			case <-s.blockTimer.C:
			case <-s.roundTimer.C:
			default:
				break cleanup
			}
		}
		s.wg.Done()
	}()

	return nil
}

// Halt SatoshiPlus service goroutine
func (s *SPService) Halt() error {
	s.Lock()
	defer s.Unlock()
	log.Info("satoshiplus Stop")

	if s.existCh != nil {
		close(s.existCh)
		s.existCh = nil
	}
	s.wg.Wait()
	return nil
}

// get validators for round
func (s *SPService) getValidators(round uint32, verbose bool) ([]*common.Address, map[common.Address]uint16, error) {
	validators, weightMap, err := s.config.Chain.GetValidators(round)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get validators %v", err.Error())
	}
	if uint16(len(validators)) != chaincfg.ActiveNetParams.RoundSize {
		return nil, nil, fmt.Errorf("getValidators error: can not get validators %d for round = %d",
			len(validators), round)
	}

	if verbose {
		for i := 0; i < len(validators); i++ {
			log.Infof("validators[%d]=%v", i, validators[i].String())
		}
	}
	return validators, weightMap, nil
}

// Initialize Consensus, it only called when service start
func (s *SPService) initializeConsensus() error {
	chainStartTime := chaincfg.ActiveNetParams.ChainStartTime

	now := time.Now().Unix()
	roundSizei64 := int64(chaincfg.ActiveNetParams.RoundSize)
	d := now - chainStartTime
	if d < common.DefaultBlockInterval {
		s.context.Round = 0
		s.context.Slot = roundSizei64 - 1
		s.context.RoundStartTime = chainStartTime - (roundSizei64-1) * common.DefaultBlockInterval
	} else {
		round0StartTime := chainStartTime - (roundSizei64-1) * common.DefaultBlockInterval
		round, slot, roundStartTime, err := s.getRoundInfo(round0StartTime, 0, now)
		if err != nil {
			log.Error("SPService initializeConsensus get round info error: ", err)
			return err
		}
		s.context.Round = round
		s.context.Slot = slot
		s.context.RoundStartTime = roundStartTime
	}
	s.resetRoundInterval(s.context.Round)

	s.resetTimer()

	log.Infof("SPService initializeConsensus round: %v, slot: %v, roundStartTime: %v", s.context.Round, s.context.Slot, s.context.RoundStartTime)
	return nil
}

//sync control of local slot:
func (s *SPService) slotControl() (int64, int64, bool) {
	slot := s.context.Slot + 1
	round := s.context.Round
	if slot == int64(chaincfg.ActiveNetParams.RoundSize) {
		s.context.RoundStartTime = s.context.RoundStartTime + s.context.RoundInterval
		slot = 0
		round = round + 1
		s.resetRoundInterval(round)
	}

	s.context.Slot = slot
	s.context.Round = round

	config := s.config
	best := config.Chain.BestSnapshot()
	if config.IsCurrent() != true && best.Height > 0 {
		log.Infof("downloading blocks: wait!!!!!!!!!!!")
		return 0, 0, false
	}

	verbose := round != s.context.Round
	validators, _, err := s.getValidators(uint32(round), verbose)
	if err != nil {
		log.Errorf("[slotControl] %v", err.Error())
		return 0, 0, false
	}

	isTurn := *validators[slot] == *s.config.Account.Address
	log.Infof("[slotControl] slot change slot=%d, round=%d, height=%d, isTurn=%v, interval=%v",
		slot, round, best.Height+1, isTurn, s.context.RoundInterval)
	return round, slot, isTurn
}

// return the interval of round
func (s *SPService) getRoundInterval(round int64) int64 {
	return s.config.RoundManager.GetRoundInterval(round)
}

// reset round interval of context, round interval need be reset when round change.
func (s *SPService) resetRoundInterval(round int64) bool {
	roundInterval := s.getRoundInterval(round)
	if roundInterval == 0 {
		log.Errorf("[genBlock] failed to adjust time")
		return false
	}
	log.Infof("Reset round interval, round %d, block interval %f", round, float64(roundInterval)/float64(chaincfg.ActiveNetParams.RoundSize))
	s.context.RoundInterval = roundInterval
	return true
}

// when the it turns to be a validator, try to generate a new block
func (s *SPService) genBlock() (*asiutil.Block, error) {
	round, slot, isTurn := s.slotControl()
	s.resetTimer()
	if !isTurn {
		return nil, nil
	}
	blockInterval := float64(s.GetRoundInterval()) / float64(chaincfg.ActiveNetParams.RoundSize) * 1000
	log.Infof("satoshiplus gen block start at round=%d, slot=%d", round, slot)

	block, err := s.config.BlockTemplateGenerator.ProduceNewBlock(
		s.config.Account, s.config.GasFloor, s.config.GasCeil, uint32(round), uint16(slot), blockInterval)
	if err != nil {
		log.Errorf("satoshiplus gen block failed to make a block: %v", err)
		return nil, err
	}

	return block, nil
}

// return round interval currently
func (s *SPService) GetRoundInterval() int64 {
	return s.context.RoundInterval
}

// getRoundInfo returns target round/slot, caculated from passing block time/round/slot.
func (s *SPService) getRoundInfo(roundTime int64, round int64, targetTime int64) (int64, int64, int64, error) {
	if roundTime > targetTime {
		return 0, 0, 0, errors.New("targetTime must be greater than block time")
	}
	log.Infof("satoshi get round info first block %v, target %v", roundTime, targetTime)

	if roundTime == targetTime {
		return round, 0, roundTime, nil
	}

	curTime := roundTime
	roundSizei64 := int64(chaincfg.ActiveNetParams.RoundSize)
	var roundInterval int64
	for {
		if curTime > targetTime {
			break
		}
		roundInterval = s.getRoundInterval(round)
		if roundInterval == 0 {
			err := fmt.Errorf("splus getRoundInfo failed to get round interval,round : %v", round)
			log.Errorf(err.Error())
			return 0, 0, 0, err
		}
		curTime += roundInterval
		round = round + 1
	}
	span := curTime - targetTime - 1
	num := (span*int64(chaincfg.ActiveNetParams.RoundSize))/roundInterval + 1
	roundStartTime := curTime - roundInterval

	return round - 1, roundSizei64 - num, roundStartTime, nil
}

// reset blockTimer.
func (s *SPService) resetTimer() {
	d := time.Duration(int64(s.context.Slot+1)*s.context.RoundInterval) * time.Second / time.Duration(chaincfg.ActiveNetParams.RoundSize)
	offset := time.Unix(s.context.RoundStartTime, 0).Add(d).Sub(time.Now())
	s.blockTimer.Reset(offset)
}

// handleBlockchainNotification handles notifications from blockchain.  It does
// things such as generate block ahead of time.
func (s *SPService) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {
	// A block has been connected to the main block chain.
	case blockchain.NTBlockConnected:
		fallthrough
	// A block has been disconnected from the main block chain.
	case blockchain.NTBlockDisconnected:
		dataList, ok := notification.Data.([]interface{})
		if !ok || len(dataList) < 3 {
			log.Warnf("Chain notification need a block node.")
			break
		}
		node, ok := dataList[2].(ainterface.BlockNode)
		if !ok {
			log.Warnf("Chain notification need a block node at the second position.")
			break
		}
		// clear chainTipChan if exist
		select {
		case <-s.chainTipChan:
		default:
		}
		go func() {
			s.chainTipChan <- node
		}()
	}
}
