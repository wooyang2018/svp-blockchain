// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/logger"
)

// PeerStatus type
type PeerStatus int8

// PeerStatus
const (
	PeerStatusDisconnected PeerStatus = iota
	PeerStatusConnecting
	PeerStatusConnected
)

const (
	// message size limit in bytes (~100 MB)
	// to avoid out of memory allocation for reading next message
	MessageSizeLimit uint32 = 100000000
)

// Peer type
type Peer struct {
	pubKey    *core.PublicKey
	pointAddr multiaddr.Multiaddr
	topicAddr multiaddr.Multiaddr
	status    PeerStatus

	mtxRWC    sync.RWMutex
	mtxStatus sync.RWMutex
	mtxWrite  sync.Mutex

	reconInterval  time.Duration
	fastReconCount int
	mtxRecon       sync.RWMutex

	rwc  io.ReadWriteCloser
	host *Host
}

func NewPeer(pubKey *core.PublicKey, pointAddr, topicAddr multiaddr.Multiaddr) *Peer {
	p := &Peer{
		pubKey:    pubKey,
		pointAddr: pointAddr,
		topicAddr: topicAddr,
		status:    PeerStatusDisconnected,
	}
	p.resetReconnectInterval()
	return p
}

// PublicKey returns public key of peer
func (p *Peer) PublicKey() *core.PublicKey {
	return p.pubKey
}

// PointAddr return point address of peer
func (p *Peer) PointAddr() multiaddr.Multiaddr {
	return p.pointAddr
}

// TopicAddr return topic address of peer
func (p *Peer) TopicAddr() multiaddr.Multiaddr {
	return p.topicAddr
}

func (p *Peer) Status() PeerStatus {
	p.mtxStatus.RLock()
	defer p.mtxStatus.RUnlock()

	return p.status
}

func (p *Peer) disconnect() {
	p.mtxStatus.Lock()
	defer p.mtxStatus.Unlock()

	if p.status == PeerStatusConnected {
		logger.I().Infow("peer disconnected", "pointAddr", p.pointAddr)
	}
	p.status = PeerStatusDisconnected
	rwc := p.getRWC()
	if rwc != nil {
		rwc.Close()
	}
	p.reconnectAfterInterval()
}

func (p *Peer) reconnectAfterInterval() {
	reconnInterval := p.increaseReconnectInterval() +
		(time.Duration(rand.Intn(500)) * time.Millisecond)

	time.AfterFunc(reconnInterval, func() {
		if p.host != nil {
			p.host.ConnectPeer(p.host.consLeader)
		}
	})
}

func (p *Peer) setConnecting() error {
	p.mtxStatus.Lock()
	defer p.mtxStatus.Unlock()

	if p.status != PeerStatusDisconnected {
		return errors.New("status must be disconnected")
	}
	p.status = PeerStatusConnecting
	logger.I().Infow("connecting", "pointAddr", p.pointAddr)
	return nil
}

func (p *Peer) onConnected(rwc io.ReadWriteCloser) {
	p.mtxStatus.Lock()
	defer p.mtxStatus.Unlock()

	logger.I().Infow("peer connected", "pointAddr", p.pointAddr)
	p.status = PeerStatusConnected
	p.setRWC(rwc)
	p.resetReconnectInterval()
	go p.listen()
}

type PointMsg struct {
	peer *Peer
	data []byte
}

func (p *Peer) listen() {
	defer p.disconnect()
	for {
		msg, err := p.read()
		if err != nil {
			return
		}
		p.host.emitter.Emit(&PointMsg{
			peer: p,
			data: msg,
		})
	}
}

func (p *Peer) read() ([]byte, error) {
	b, err := p.readFixedSize(4)
	if err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(b)
	if size > MessageSizeLimit {
		return nil, fmt.Errorf("big message size %d", size)
	}
	return p.readFixedSize(size)
}

func (p *Peer) readFixedSize(size uint32) ([]byte, error) {
	b := make([]byte, size)
	_, err := io.ReadFull(p.getRWC(), b)
	return b, err
}

func (p *Peer) WriteMsg(msg []byte) error {
	p.mtxWrite.Lock()
	defer p.mtxWrite.Unlock()

	if p.Status() != PeerStatusConnected {
		p.host.ConnectPeer(p)
		return errors.New("peer not connected")
	}
	return p.write(msg)
}

func (p *Peer) write(b []byte) error {
	payload := make([]byte, 4, 4+len(b))
	binary.BigEndian.PutUint32(payload, uint32(len(b)))
	payload = append(payload, b...)

	_, err := p.getRWC().Write(payload)
	return err
}

func (p *Peer) setRWC(rwc io.ReadWriteCloser) {
	p.mtxRWC.Lock()
	defer p.mtxRWC.Unlock()
	p.rwc = rwc
}

func (p *Peer) getRWC() io.ReadWriteCloser {
	p.mtxRWC.RLock()
	defer p.mtxRWC.RUnlock()
	return p.rwc
}

func (p *Peer) resetReconnectInterval() {
	p.mtxRecon.Lock()
	defer p.mtxRecon.Unlock()
	p.reconInterval = 1 * time.Second
	p.fastReconCount = 3
}

func (p *Peer) increaseReconnectInterval() time.Duration {
	p.mtxRecon.Lock()
	defer p.mtxRecon.Unlock()

	if p.fastReconCount > 0 {
		p.fastReconCount--
		return p.reconInterval
	}
	p.reconInterval *= 2
	maxInterval := 32 * time.Second
	if p.reconInterval > maxInterval {
		p.reconInterval = maxInterval
	}
	return p.reconInterval
}
