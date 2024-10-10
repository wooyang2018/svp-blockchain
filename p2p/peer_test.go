// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/wooyang2018/svp-blockchain/emitter"
)

type rwcLoopBack struct {
	buf      *bytes.Buffer
	closedCh chan struct{}
	inCh     chan struct{}
}

func newRWCLoopBack() *rwcLoopBack {
	return &rwcLoopBack{
		buf:      bytes.NewBuffer(nil),
		closedCh: make(chan struct{}),
		inCh:     make(chan struct{}, 2),
	}
}

func (rwc *rwcLoopBack) Read(b []byte) (n int, err error) {
	select {
	case <-rwc.closedCh:
		return 0, io.EOF
	default:
		n, err = rwc.readBuf(b)
		if err == io.EOF {
			select {
			case <-rwc.closedCh:
			case <-rwc.inCh:
				return rwc.Read(b)
			}
		}
		return n, err
	}
}

func (rwc *rwcLoopBack) readBuf(b []byte) (int, error) {
	return rwc.buf.Read(b)
}

func (rwc *rwcLoopBack) Write(b []byte) (n int, err error) {
	select {
	case <-rwc.closedCh:
		return 0, io.EOF
	default:
	}
	n, err = rwc.buf.Write(b)
	select {
	case rwc.inCh <- struct{}{}:
	default:
	}
	return n, err
}

func (rwc *rwcLoopBack) Close() error {
	select {
	case <-rwc.closedCh:
		return io.EOF
	default:
		close(rwc.closedCh)
		rwc.buf.Reset()
		return nil
	}
}

func TestRWCLoopBack(t *testing.T) {
	asrt := assert.New(t)

	rwc := newRWCLoopBack()
	var recv SafeBytes
	go func() {
		for {
			data := make([]byte, 5)
			rwc.Read(data)
			recv.Set(data)
		}
	}()

	sent := []byte("hello")
	rwc.Write(sent)

	time.Sleep(time.Millisecond)
	asrt.EqualValues(sent, recv.Get())

	rwc.Close()
	_, err := rwc.Write(sent)
	asrt.Error(err)
}

type MockListener struct {
	mock.Mock
}

func (m *MockListener) CB(e emitter.Event) {
	m.Called(e)
}

func TestPeerReadWrite(t *testing.T) {
	asrt := assert.New(t)
	p := NewPeer(nil, nil, nil)

	rwc := newRWCLoopBack()
	p.onConnected(rwc)
	sub := p.SubscribeMsg()
	msg := []byte("hello")

	mln := new(MockListener)
	mln.On("CB", msg).Once()

	go func() {
		for event := range sub.Events() {
			mln.CB(event)
		}
	}()

	asrt.NoError(p.WriteMsg(msg))
	time.Sleep(time.Millisecond)
	mln.AssertExpectations(t)
}

func TestPeerConnStatus(t *testing.T) {
	asrt := assert.New(t)
	p := NewPeer(nil, nil, nil)

	asrt.Equal(PeerStatusDisconnected, p.Status())

	rwc := newRWCLoopBack()
	p.onConnected(rwc)

	asrt.Equal(PeerStatusConnected, p.Status())

	rwc.Close()
	time.Sleep(time.Millisecond)

	asrt.Equal(PeerStatusDisconnected, p.Status())

	p = NewPeer(nil, nil, nil)
	err := p.setConnecting()

	asrt.NoError(err)
	asrt.Equal(PeerStatusConnecting, p.Status())

	p.disconnect()
	asrt.Equal(PeerStatusDisconnected, p.Status())

	p.onConnected(newRWCLoopBack())
	err = p.setConnecting()

	asrt.Error(err)
	asrt.Equal(PeerStatusConnected, p.Status())
}
