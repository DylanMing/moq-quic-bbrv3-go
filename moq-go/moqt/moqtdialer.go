package moqt

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"
)

type DialerOptions struct {
	QuicConfig         *quic.Config
	ALPNs              []string
	InsecureSkipVerify bool
}

type MOQTDialer struct {
	Options DialerOptions
	Role    uint64
	Ctx     context.Context
}

func (d *MOQTDialer) Dial(addr string) (*MOQTSession, error) {

	Options := d.Options

	// 使用默认的 BBRv3 配置
	quicConfig := Options.QuicConfig
	if quicConfig == nil {
		quicConfig = &quic.Config{
			Congestion: func() quic.SendAlgorithmWithDebugInfos {
				return quic.NewBBRv3(nil)
			},
		}
	}

	tlsConfig := tls.Config{
		NextProtos:         Options.ALPNs,
		InsecureSkipVerify: Options.InsecureSkipVerify,
	}

	conn, err := quic.DialAddr(d.Ctx, addr, &tlsConfig, quicConfig)

	if err != nil {
		return nil, err
	}

	session, err := CreateMOQSession(conn, d.Role, CLIENT_MODE)

	if err != nil {
		return nil, err
	}

	session.ServeMOQ()

	timeout := time.After(time.Second * 5)

	select {
	case <-session.HandshakeDone:
		return session, nil
	case <-timeout:
		return nil, fmt.Errorf("[Error Dialing MOQT][Timeout]")
	}
}
