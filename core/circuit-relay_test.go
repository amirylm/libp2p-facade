package core

import (
	"context"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/pnet"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"

	"sync"
	"testing"
)

// - create circuit-relay node
// - create node 1, connect to relayer
// - create node 2 + stream handler, connect to relayer
// - node 1 connect to node 2 via circuit-relay
func TestRelayer(t *testing.T) {
	var wg sync.WaitGroup

	psk := PNetSecret()

	priv, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 1)
	rel := NewRelayer(context.Background(), NewConfig(priv, psk, nil))
	go AutoClose(rel.Context(), rel)

	err := rel.DHT().Bootstrap(context.Background())
	assert.Nil(t, err)
	relInfo := peer.AddrInfo{
		ID:    rel.Host().ID(),
		Addrs: rel.Host().Addrs(),
	}

	addr1, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/3031")
	n1 := newPeer(psk, []multiaddr.Multiaddr{addr1})
	go AutoClose(n1.Context(), n1)

	wg.Add(1)
	go func() {
		conns := Connect(n1, []peer.AddrInfo{relInfo}, true)
		for conn := range conns {
			n1.Logger().Infof("new connection "+conn.Info.ID.String()+", error: ", conn.Error)
		}
		wg.Done()
	}()
	wg.Wait()

	addr2, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/3032")
	n2 := newPeer(psk, []multiaddr.Multiaddr{addr2})
	go AutoClose(n2.Context(), n2)

	n2.Host().SetStreamHandler("/hello", func(s network.Stream) {
		wg.Done()
		err := s.Close()
		assert.Nil(t, err)
	})
	// n2 -> rel
	wg.Add(1)
	go func() {
		conns := Connect(n2, []peer.AddrInfo{relInfo}, true)
		for conn := range conns {
			n2.Logger().Info("connect event:", conn)
		}
		wg.Done()
	}()
	wg.Wait()

	// n1 -> relay -> n2
	n2relayInfo := CircuitRelayAddrInfo(rel.Host().ID(), n2.Host().ID())
	wg.Add(1)
	go func() {
		conns := Connect(n1, []peer.AddrInfo{n2relayInfo}, false)
		for conn := range conns {
			n1.Logger().Info("connect event:", conn)
		}
		wg.Done()
	}()
	wg.Wait()

	wg.Add(1)
	s, err := n1.Host().NewStream(context.Background(), n2.Host().ID(), "/hello")
	assert.Nil(t, err, "can't send message: %s", err)
	_, err = s.Read(make([]byte, 1)) // block until the handler closes the stream
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "EOF")
	wg.Wait()
}

func newPeer(psk pnet.PSK, addrs []multiaddr.Multiaddr) LibP2PPeer {
	priv, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 1)
	ropts := NewConfig(priv, psk, nil)
	ropts.Addrs = addrs
	return NewBasePeer(context.Background(), ropts, libp2p.EnableRelay())
}
