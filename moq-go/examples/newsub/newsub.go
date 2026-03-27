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

var ALPNS = []string{"moq-00"} // Application Layer Protocols ["H3" - WebTransport]
//const RELAY = "192.168.1.10:4443"

const RELAY = "127.0.0.1:4444"

func main() {
    go func() {
       http.ListenAndServe(":8080", nil)
    }()

    debug := flag.Bool("debug", false, "sets log level to debug")
    flag.Parse()

    zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
       return filepath.Base(file) + ":" + strconv.Itoa(line)
    }

    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.StampMilli}).With().Caller().Logger()
    zerolog.SetGlobalLevel(zerolog.DebugLevel)

    if *debug {
       zerolog.SetGlobalLevel(zerolog.DebugLevel)
    }

    Options := moqt.DialerOptions{
       ALPNs: ALPNS,
       QuicConfig: &quic.Config{
          KeepAlivePeriod: 1 * time.Second,
          EnableDatagrams: true,
          MaxIdleTimeout:  60 * time.Second,
             Congestion: func() quic.SendAlgorithmWithDebugInfos {
                return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "newsub"))
             },
       },
       InsecureSkipVerify: true,
    }

    sub := api.NewMOQSub(Options, RELAY)

    handler, err := sub.Connect()

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
    //handler.Subscribe("bbb", "default", 0)
    handler.Subscribe("bbb", "dumeel", 0)
    //handler.SubscribeFrom("bbb", "dumeel", 0, 2, 30)
    log.Debug().Msgf("sub main after Subscribe %s", sub)
    <-sub.Ctx.Done()
}

func handleStream(ss *moqt.SubStream) {

    log.Debug().Msgf("New Stream Header")

    for moqtstream := range ss.StreamsChan {
       //log.Debug().Msgf("New Group Stream - %+v", moqtstream)
       go handleMOQStream(moqtstream)
    }
}

func handleMOQStream(stream wire.MOQTStream) {
    //log.Debug().Msgf("handleMOQStream: start reading from stream %+v", stream)

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

    //log.Printf("handleMOQStream: stream ended")
}