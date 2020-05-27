// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package solo

import (
	"errors"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/consensus/params"
	"sync"
	"time"
)

/*
 * Simple consensus for solo node in test environment.
 */
type SoloService struct {
	sync.Mutex
	wg      sync.WaitGroup
	existCh chan interface{}
	context params.Context
	timer   *time.Timer
	config  *params.Config
}

/*
 * create Solo Service
 */
func NewSoloService(config *params.Config) (*SoloService, error) {
	service := &SoloService{
		config: config,
	}
	return service, nil
}

func (s *SoloService) Start() error {
	s.Lock()
	defer s.Unlock()
	if s.existCh != nil {
		log.Info("consensus have started")
		return errors.New("consensus already started")
	}
	if s.config.Account == nil {
		log.Warn("solo service exit when account is nil")
		return nil
	}

	// temp timer, need reset
	s.timer = time.NewTimer(common.DefaultBlockInterval * 10000)
	s.initializeConsensus()
	s.existCh = make(chan interface{})
	s.wg.Add(1)
	go func() {
		defer s.timer.Stop()
		existCh := s.existCh
		for {
			select {
			case <-s.timer.C:
				s.genBlock()
			case <-existCh:
				s.wg.Done()
				return
			}
		}
	}()
	return nil
}

func (s *SoloService) Halt() error {
	s.Lock()
	defer s.Unlock()
	if s.existCh != nil {
		close(s.existCh)
		s.existCh = nil
	}
	s.wg.Wait()
	return nil
}

func (s *SoloService) genBlock() {
	round, slot := s.slotControl()
	s.resetTimer()
	blockInterval := float64(s.GetRoundInterval()) / float64(chaincfg.ActiveNetParams.RoundSize) * 1000

	// Create a new block using the available transactions
	// in the memory pool as a source of transactions to potentially
	// include in the block.
	template, err := s.config.BlockTemplateGenerator.ProduceNewBlock(
		s.config.Account, s.config.GasFloor, s.config.GasCeil,
		time.Now().Unix(), uint32(round), uint16(slot), blockInterval)
	if err != nil {
		log.Errorf("solo failed to create new block:%s", err)
		return
	}

	_, err = s.config.ProcessBlock(template.Block, template.VBlock, common.BFFastAdd)
	if err != nil {
		// Anything other than a rule violation is an unexpected error,
		// so log that error as an internal error.
		if _, ok := err.(blockchain.RuleError); !ok {
			log.Errorf("Solo gen block rejected unexpected: %v", err)
			return
		}
		log.Errorf("Solo gen block rejected: %v", err)
		return
	}
	log.Infof("Solo gen block accept, height=%d, hash=%v, sigNum=%v, txNum=%d",
		template.Block.Height(), template.Block.Hash(),
		len(template.Block.MsgBlock().PreBlockSigs), len(template.Block.Transactions()))
}

func (s *SoloService) GetRoundInterval() int64 {
	return s.context.RoundInterval
}

//sync control of local slot:
func (s *SoloService) slotControl() (int64, int64) {

	duration := time.Now().Unix() - s.context.RoundStartTime
	roundsizei64 := int64(chaincfg.ActiveNetParams.RoundSize)
	slotCount := duration * roundsizei64 / s.context.RoundInterval

	if slotCount < roundsizei64 {
		s.context.Slot = slotCount
	} else {
		roundCount := slotCount / roundsizei64
		s.context.Round += roundCount
		s.context.Slot = slotCount % roundsizei64
		s.context.RoundStartTime = s.context.RoundStartTime + s.context.RoundInterval * roundCount
	}
	best := s.config.Chain.BestSnapshot()
	if s.config.IsCurrent() != true {
		log.Infof("waiting blocks")
		return 0, 0
	}

	log.Infof("slotControl change slot=%d, round=%d, height=%d", s.context.Slot, s.context.Round, best.Height+1)
	return s.context.Round, s.context.Slot
}

func (s *SoloService) initializeConsensus() {

	s.context.RoundInterval = common.DefaultBlockInterval * int64(chaincfg.ActiveNetParams.RoundSize)
	chainStartTime := chaincfg.ActiveNetParams.ChainStartTime

	d := time.Now().Unix() - chainStartTime
	roundSizei64 := int64(chaincfg.ActiveNetParams.RoundSize)
	if d < common.DefaultBlockInterval {
		s.context.Round = 0
		s.context.Slot = roundSizei64 - 1
		s.context.RoundStartTime = chainStartTime - (roundSizei64 - 1) * common.DefaultBlockInterval
	} else {
		// blockinterval = RoundInterval / RoundSize
		// slotCount = d / blockinterval
		slotCount := d * roundSizei64 / s.context.RoundInterval
		s.context.Round = 1 + slotCount / roundSizei64
		s.context.Slot = slotCount % roundSizei64
		s.context.RoundStartTime = chainStartTime + common.DefaultBlockInterval + s.context.RoundInterval * (s.context.Round-1)
	}
	log.Infof("Solo initializeConsensus round: %v, slot: %v, roundStartTime: %v",
		s.context.Round, s.context.Slot, s.context.RoundStartTime)
	s.resetTimer()
}

func (s *SoloService) resetTimer() {
	d := time.Duration(int64(s.context.Slot+1) * s.context.RoundInterval) * time.Second / time.Duration(chaincfg.ActiveNetParams.RoundSize)
	offset := time.Unix(s.context.RoundStartTime, 0).Add(d + time.Millisecond).Sub(time.Now())
	s.timer.Reset(offset)
}
