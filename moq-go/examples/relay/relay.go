package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/DineshAdhi/moq-go/moqt"
	"github.com/DineshAdhi/moq-go/moqt/api"

	"github.com/quic-go/quic-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const PORT = 4443

var ALPNS = []string{"h3", "moq-00"} // Application Layer Protocols ["H3" - WebTransport]

var defaultCertPath = "examples/certs/localhost.crt"
var defaultKeyPath = "examples/certs/localhost.key"

func main() {
	// defer profile.Start(profile.ProfilePath("."), profile.GoroutineProfile, profile.MemProfileHeap, profile.CPUProfile).Stop()

	go func() {
		http.ListenAndServe(":8080", nil)
	}()

	debug := flag.Bool("debug", false, "sets log level to debug")
	port := flag.Int("port", PORT, "Listening Port")
	KEYPATH := flag.String("keypath", defaultKeyPath, "Keypath")
	CERTPATH := flag.String("certpath", defaultCertPath, "CertPath")
	flag.Parse()

	LISTENADDR := fmt.Sprintf("0.0.0.0:%d", *port)

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.StampMilli}).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	quicConfig := &quic.Config{
		EnableDatagrams: true,
		Congestion: func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewBBRv3(nil)
		},
	}

	Options := moqt.ListenerOptions{
		ListenAddr: LISTENADDR,
		CertPath:   *CERTPATH,
		KeyPath:    *KEYPATH,
		ALPNs:      ALPNS,
		QuicConfig: quicConfig,
	}

	peers := []string{} // TODO : Address of the Relay Peers for Fan out Implementation

	relay := api.NewMOQTRelay(Options, peers)
	relay.Run()
}
