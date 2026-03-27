package main

import (
	"flag"
	"math/rand"
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

const PORT = 4443
const STATS_INTERVAL = 1 * time.Second

var ALPNS = []string{"moq-00"}
const RELAY = "127.0.0.1:4443"

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
		log.Info().Msg("Using Cubic congestion control (default)")
		congestionFunc = nil
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

	pub := api.NewMOQPub(Options, RELAY)
	log.Debug().Msgf("pub main before Connect %s", pub)
	handler, err := pub.Connect()
	log.Debug().Msgf("pub main after Connect %s", pub)

	var statsLogger *moqt.StatsLogger
	if *stats {
		statsLogger = moqt.NewStatsLogger(pub, STATS_INTERVAL)
		statsLogger.Start()
	}

	pub.OnSubscribe(func(ps moqt.PubStream) {
		log.Debug().Msgf("New Subscribe Request - %s", ps.TrackName)
		go handleStream(&ps)
	})

	if err != nil {
		log.Error().Msgf("error - %s", err)
		return
	}
	log.Debug().Msgf("pub main before SendAnnounce %s", pub)
	handler.SendAnnounce("bbb")
	log.Debug().Msgf("pub main after SendAnnounce %s", pub)

	if statsLogger != nil {
		time.Sleep(30 * time.Second)
		statsLogger.Stop()
	} else {
		<-pub.Ctx.Done()
	}
}

func handleStream(stream *moqt.PubStream) {
	stream.Accept()
	groupid1 := uint64(0)
	for {
		gs1, err := stream.NewGroup(groupid1)
		if err != nil {
			log.Error().Msgf("Err - %s", err)
			return
		}

		objectid := uint64(0)

		for range 100 {
			length := 204800 + rand.Intn(1024)
			timestamp := time.Now().Format("2006-01-02 15:04:05.999999")
			payload1 := make([]byte, length)
			copy(payload1, []byte("gs1 "+timestamp))
			for i := len(timestamp); i < length; i++ {
				payload1[i] = 'A'
			}

			gs1.WriteObject(&wire.Object{
				GroupID: groupid1,
				ID:      objectid,
				Payload: payload1,
			})
			log.Printf("WriteObject groupid： %d, objectid %d, timestamp %s, length %d", groupid1, objectid, timestamp, length)
			objectid++
			time.Sleep(10 * time.Millisecond)
		}

		gs1.Close()
		groupid1 = groupid1 + 1
	}
}
