package lib

import (
	"context"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/pnet"
	secio "github.com/libp2p/go-libp2p-secio"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
	"github.com/multiformats/go-multiaddr"
)

// ConfigureLibp2pOpts is the custom config hook interface
type ConfigureLibp2pOpts = func([]libp2p.Option) ([]libp2p.Option, error)

// Options holds the needed configuration for creating a private node instance
type Options struct {
	Ctx context.Context
	// PrivKey of the current node
	PrivKey crypto.PrivKey
	// Secret is the private network secret ([32]byte)
	Secret pnet.PSK
	// Addrs are Multiaddrs for the current node, will fallback to libp2p defaults
	Addrs []multiaddr.Multiaddr
	// Logger to use (see github.com/ipfs/go-log/v2)
	// will fallback to defaultLogger()
	Logger logging.EventLogger
	// DS is the data store used by DHT
	DS datastore.Batching
	// UseLibp2pOpts is a hook for configuring custom libp2p options
	UseLibp2pOpts ConfigureLibp2pOpts
	// Discovery contains configuration of discovery services
	// unless Discovery is nil, local mdns will be added by default
	Discovery *DiscoveryOptions
	// Peers are nodes that we want to connect on bootstrap
	Peers []peer.AddrInfo
}

// NewOptions creates the minimum needed Options
func NewOptions(priv crypto.PrivKey, psk pnet.PSK, discOpts *DiscoveryOptions) *Options {
	opts := Options{
		Ctx:     context.Background(),
		PrivKey: priv,
		Secret:  psk,
	}
	if discOpts != nil {
		opts.Discovery = discOpts
	}
	return &opts
}

// ToLibP2pOpts converts Options into the corresponding []libp2p.Option
func (opts *Options) ToLibP2pOpts() ([]libp2p.Option, error) {
	err := opts.defaults()
	if err != nil {
		return nil, err
	}
	libp2pOpts := []libp2p.Option{
		libp2p.Identity(opts.PrivKey),
		libp2p.ListenAddrs(opts.Addrs...),
		libp2p.PrivateNetwork(opts.Secret),
		libp2p.NATPortMap(),
		libp2p.EnableAutoRelay(),
		libp2p.EnableNATService(),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(secio.ID, secio.New),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
	}
	finalOpts, err := opts.UseLibp2pOpts(libp2pOpts)
	return finalOpts, err
}

func (opts *Options) defaults() error {
	if opts.PrivKey == nil {
		priv, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 1)
		opts.PrivKey = priv
	}
	if opts.Secret == nil {
		opts.Secret = PNetSecret()
	}
	if opts.Addrs == nil || len(opts.Addrs) == 0 {
		// currently using libp2p defaults, might be changed
		addripv4, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		addripv6, _ := multiaddr.NewMultiaddr("/ip6/::/tcp/0")
		opts.Addrs = []multiaddr.Multiaddr{addripv4, addripv6}
	}
	if opts.Logger == nil {
		opts.Logger = defaultLogger()
	}
	if opts.UseLibp2pOpts == nil {
		opts.UseLibp2pOpts = func(_opts []libp2p.Option) ([]libp2p.Option, error) {
			return _opts, nil
		}
	}
	return nil
}
