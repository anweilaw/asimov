// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// prove of authority.
package poa

import (
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/consensus/params"
	"sync"
	"time"
)

// prove of authority.
// using round robin algorithm to generate blocks with orders in the list of authority peers.
// this consensus is used by sub chain using asimov.
// please refer to the white paper of asimov.
type Service struct {
	sync.Mutex
	wg      sync.WaitGroup
	existCh chan interface{}
	context params.Context
	timer   *time.Timer

	started bool
	config  *params.Config
}

/*
 * create POA Service
 */
func NewService(config *params.Config) (*Service, error) {
	if config == nil {
		return nil, errors.New("config can't be nil")
	}
	if config.Chain == nil {
		return nil, errors.New("config.chain can't be nil")
	}
	service := &Service{
		config: config,
	}
	return service, nil
}

/*
 * start the Service
 */
func (s *Service) Start() error {
	s.Lock()
	defer s.Unlock()
	if s.existCh != nil {
		return errors.New("region consensus is already started")
	}
	if s.config.Account == nil {
		log.Warn("region service exit when account is nil")
		return nil
	}
	log.Info("Region consensus start")

	// current block maybe do not at the best block height:
	s.timer = time.NewTimer(common.DefaultBlockInterval * 1000000 * time.Second)
	if err := s.initializeConsensus(); err != nil {
		log.Errorf("Start poa service failed:", err)
		s.timer.Stop()
		s.timer = nil
		return err
	}
	s.started = true

	s.existCh = make(chan interface{})
	s.wg.Add(1)
	go func() {
		existCh := s.existCh
	mainloop:
		for {
			select {
			case <-s.timer.C:
				s.genBlock()
			case <-existCh:
				break mainloop
			}
		}
		s.timer.Stop()
	cleanup:
		for {
			select {
			case <-s.timer.C:
			default:
				break cleanup
			}
		}
		s.wg.Done()
	}()

	return nil
}

func (s *Service) Halt() error {
	s.Lock()
	defer s.Unlock()
	log.Info("Region Service Stop")

	if s.existCh != nil {
		close(s.existCh)
		s.existCh = nil
	}
	s.wg.Wait()
	s.started = false
	return nil
}

func (s *Service) isStarted() bool {
	s.Lock()
	defer s.Unlock()
	return s.started
}

//generate the validators of current round according to the last state of the chain.
func (s *Service) getValidators(round uint32, verbose bool) ([]*common.Address, map[common.Address]uint16, error) {
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

/*
 * Initialize Consensus
 */
func (s *Service) initializeConsensus() error {
	s.context.RoundInterval = common.DefaultBlockInterval * int64(chaincfg.ActiveNetParams.RoundSize)
	chainStartTime := int64(chaincfg.ActiveNetParams.ChainStartTime)

	d := time.Now().Unix() - chainStartTime
	roundSizei64 := int64(chaincfg.ActiveNetParams.RoundSize)
	if d < common.DefaultBlockInterval {
		s.context.Round = 0
		s.context.Slot = roundSizei64 - 1
	} else {
		slotCount := d * roundSizei64 / s.context.RoundInterval
		s.context.Round = 1 + slotCount/roundSizei64
		s.context.Slot = slotCount % roundSizei64
	}
	s.context.RoundStartTime = chainStartTime + common.DefaultBlockInterval + s.context.RoundInterval*int64(s.context.Round-1)

	s.resetTimer(false)

	log.Infof("POA initializeConsensus round: %v, slot: %v, roundStartTime: %v", s.context.Round, s.context.Slot, s.context.RoundStartTime)
	return nil
}

//sync control of local slot:
func (s *Service) slotControl() (int64, int64, bool) {
	config := s.config
	best := config.Chain.BestSnapshot()
	if config.IsCurrent() != true && best.Height > 0 {
		log.Infof("downloading blocks: wait!!!!!!!!!!!")
		return 0, 0, false
	}

	slot := s.context.Slot + 1
	round := s.context.Round
	roundSizei64 := int64(chaincfg.ActiveNetParams.RoundSize)
	if slot == roundSizei64 {
		s.context.RoundStartTime = s.context.RoundStartTime + s.context.RoundInterval
		slot = 0
		round = round + 1
	}

	verbose := round != s.context.Round
	validators, _, err := s.getValidators(uint32(round), verbose)
	if err != nil {
		log.Errorf("[slotControl] %v", err.Error())
		return 0, 0, false
	}

	isTurn := *validators[slot] == *s.config.Account.Address
	log.Infof("[slotControl] slot change slot=%d, round=%d, height=%d, isTurn=%v, interval=%v",
		slot, round, best.Height+1, isTurn,
		float64(s.context.RoundInterval)/float64(chaincfg.ActiveNetParams.RoundSize))
	s.context.Slot = slot
	s.context.Round = round
	return round, slot, isTurn
}

// validators create block.
func (s *Service) genBlock() {
	round, slot, isTurn := s.slotControl()
	s.resetTimer(round == 0)
	if !isTurn {
		return
	}
	blockInterval := float64(s.GetRoundInterval()) / float64(chaincfg.ActiveNetParams.RoundSize) * 1000
	log.Infof("try to gen block at round=%d, slot=%d", round, slot)

	template, err := s.config.BlockTemplateGenerator.ProduceNewBlock(
		s.config.Account, s.config.GasFloor, s.config.GasCeil,
		time.Now().Unix(), uint32(round), uint16(slot), blockInterval)
	if err != nil {
		log.Errorf("Consensus POA Failed to gen a block: %v", err)
		return
	}

	_, err = s.config.ProcessBlock(template, common.BFFastAdd)
	if err != nil {
		// Anything other than a rule violation is an unexpected error,
		// so log that error as an internal error.
		if _, ok := err.(blockchain.RuleError); !ok {
			log.Errorf("POA gen block rejected unexpected: %v", err)
			return
		}
		log.Errorf("POA gen block rejected: %v", err)
		return
	}

	log.Infof("POA gen block accept, height=%d, hash=%v, sigNum=%v, txNum=%d",
		template.Block.Height(), template.Block.Hash(),
		len(template.Block.MsgBlock().PreBlockSigs), len(template.Block.Transactions()))
}

func (s *Service) GetRoundInterval() int64 {
	return s.context.RoundInterval
}

func (s *Service) resetTimer(isErr bool) {
	if isErr {
		s.timer.Reset(time.Second)
		return
	}
	d := time.Duration(int64(s.context.Slot+1)*s.context.RoundInterval) * time.Second / time.Duration(chaincfg.ActiveNetParams.RoundSize)
	offset := time.Unix(s.context.RoundStartTime, 0).Add(d + time.Millisecond).Sub(time.Now())
	s.timer.Reset(offset)
}
