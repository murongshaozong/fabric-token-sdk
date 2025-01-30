/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package lookup

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/fabric"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/network/common/rws/translator"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap/zapcore"
)

type ChannelListenerManagerConfig struct {
	MaxRetries        int
	RetryWaitDuration time.Duration
}

type channelBasedFLMProvider struct {
	fnsp           *fabric.NetworkServiceProvider
	tracerProvider trace.TracerProvider
	keyTranslator  translator.KeyTranslator
	config         ChannelListenerManagerConfig
}

func NewChannelBasedFLMProvider(fnsp *fabric.NetworkServiceProvider, tracerProvider trace.TracerProvider, keyTranslator translator.KeyTranslator, config ChannelListenerManagerConfig) *channelBasedFLMProvider {
	return &channelBasedFLMProvider{
		fnsp:           fnsp,
		tracerProvider: tracerProvider,
		keyTranslator:  keyTranslator,
		config:         config,
	}
}

func (p *channelBasedFLMProvider) NewManager(network, channel string) (ListenerManager, error) {
	net, err := p.fnsp.FabricNetworkService(network)
	if err != nil {
		return nil, err
	}
	ch, err := net.Channel(channel)
	if err != nil {
		return nil, err
	}
	return &channelBasedLLM{
		network:   network,
		channel:   ch,
		listeners: make(map[string]*Scanner),
	}, nil
}

type channelBasedLLM struct {
	network string
	channel *fabric.Channel

	listenersMutex sync.RWMutex
	listeners      map[string]*Scanner
}

func (c *channelBasedLLM) AddLookupListener(namespace string, key string, startingTxID string, stopOnLastTx bool, listener LookupListener) error {
	s := &Scanner{
		context:      nil,
		channel:      c.channel,
		namespace:    namespace,
		key:          key,
		startingTxID: startingTxID,
		stopOnLastTx: stopOnLastTx,
		listener:     listener,
	}
	c.listenersMutex.Lock()
	c.listeners[key] = s
	c.listenersMutex.Unlock()
	go s.Scan()
	return nil
}

func (c *channelBasedLLM) RemoveLookupListener(id string, listener LookupListener) error {
	c.listenersMutex.Lock()
	defer c.listenersMutex.Unlock()

	s, ok := c.listeners[id]
	if ok {
		s.Stop()
		delete(c.listeners, id)
	}
	return nil
}

type Scanner struct {
	context      context.Context
	channel      *fabric.Channel
	namespace    string
	key          string
	startingTxID string
	stopOnLastTx bool
	listener     LookupListener

	stopMutex sync.RWMutex
	stop      bool
}

func (s *Scanner) Scan() {
	v := s.channel.Vault()
	var lastTxID string
	if s.stopOnLastTx {
		id, err := v.GetLastTxID()
		if err != nil {
			return
		}
		lastTxID = id
	}

	var keyValue []byte
	if err := s.channel.Delivery().Scan(s.context, s.startingTxID, func(tx *fabric.ProcessedTransaction) (bool, error) {
		s.stopMutex.RLock()
		stop := s.stop
		s.stopMutex.RUnlock()
		if stop {
			return true, nil
		}

		logger.Debugf("scanning [%s]...", tx.TxID())

		rws, err := v.InspectRWSet(tx.Results())
		if err != nil {
			return false, err
		}

		found := false
		for _, ns := range rws.Namespaces() {
			if ns == s.namespace {
				found = true
				break
			}
		}
		if !found {
			logger.Debugf("scanning [%s] does not contain namespace [%s]", tx.TxID(), s.namespace)
			return false, nil
		}

		ns := s.namespace
		for i := 0; i < rws.NumWrites(ns); i++ {
			k, v, err := rws.GetWriteAt(ns, i)
			if err != nil {
				return false, err
			}
			if k == s.key {
				keyValue = v
				return true, nil
			}
		}
		logger.Debugf("scanning for key [%s] on [%s] not found", s.key, tx.TxID())
		if s.stopOnLastTx && lastTxID == tx.TxID() {
			logger.Debugf("final transaction reached on [%s]", tx.TxID())
		}
		return false, nil
	}); err != nil {
		logger.Errorf("failed scanning for key [%s]: [%s]", s.key, err)
	}

	if logger.IsEnabledFor(zapcore.DebugLevel) {
		logger.Debugf("scanning for key [%s] found [%s]",
			s.key,
			base64.StdEncoding.EncodeToString(keyValue),
		)
	}
	s.listener.OnStatus(s.context, s.key, keyValue)
}

func (s *Scanner) Stop() {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	s.stop = true
}
