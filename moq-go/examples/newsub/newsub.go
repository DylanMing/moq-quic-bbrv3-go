package main

import (
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/DineshAdhi/moq-go/moqt"
	"github.com/DineshAdhi/moq-go/moqt/api"
	"github.com/DineshAdhi/moq-go/moqt/wire"
	"github.com/quic-go/quic-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var ALPNS = []string{"moq-00"}
const RELAY = "127.0.0.1:4443"
const STATS_INTERVAL = 1 * time.Second

func main() {
	go func() {
		http.ListenAndServe(":8080", nil)
	}()

	debug := flag.Bool("debug", false, "sets log level to debug")
	stats := flag.Bool("stats", false, "enable statistics logging")
	congestion := flag.String("congestion", "bbr3", "congestion control: cubic, bbr1, bbr3")
	flag.Parse()

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.StampMilli}).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	var congestionFunc func() quic.SendAlgorithmWithDebugInfos
	switch *congestion {
	case "cubic":
		log.Info().Msg("Using Cubic congestion control")
		congestionFunc = func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewCubic(nil)
		}
	case "bbr1":
		log.Info().Msg("Using BBRv1 congestion control")
		congestionFunc = func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewBBRv1(nil)
		}
	case "bbr3":
		log.Info().Msg("Using BBRv3 congestion control")
		congestionFunc = func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewBBRv3(nil)
		}
	default:
		log.Error().Msgf("Unknown congestion control: %s", *congestion)
		return
	}

	Options := moqt.DialerOptions{
		ALPNs: ALPNS,
		QuicConfig: &quic.Config{
			KeepAlivePeriod: 1 * time.Second,
			EnableDatagrams: true,
			MaxIdleTimeout:  60 * time.Second,
			Congestion:      congestionFunc,
		},
	}

	sub := api.NewMOQSub(Options, RELAY)

	handler, err := sub.Connect()

	var statsLogger *moqt.StatsLogger
	if *stats {
		statsLogger = moqt.NewStatsLogger(sub, STATS_INTERVAL)
		statsLogger.Start()
	}

	sub.OnStream(func(ss moqt.SubStream) {
		go handleStream(&ss)
	})

	sub.OnAnnounce(func(ns string) {
		handler.Subscribe(ns, "dumeel", 0)
	})

	if err != nil {
		log.Error().Msgf("Error - %s", err)
		return
	}
	log.Debug().Msgf("sub main before Subscribe %s", sub)
	handler.Subscribe("bbb", "dumeel", 0)
	log.Debug().Msgf("sub main after Subscribe %s", sub)

	if statsLogger != nil {
		time.Sleep(30 * time.Second)
		statsLogger.Stop()
	} else {
		<-sub.Ctx.Done()
	}
}

func handleStream(ss *moqt.SubStream) {

	log.Debug().Msgf("New Stream Header")

	for moqtstream := range ss.StreamsChan {
		go handleMOQStream(moqtstream)
	}
}

func handleMOQStream(stream wire.MOQTStream) {
	for {
		groupid, object, err := stream.ReadObject()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Error().Msgf("handleMOQStream: error reading object - %s", err)
			break
		}

		msg := string(object.Payload[:24])
		if object.ID == 0 || object.ID == 99 {
			log.Printf("handleMOQStream Payload - %s %d %d %d ", msg, groupid, object.ID, len(object.Payload))
		}
	}
}
