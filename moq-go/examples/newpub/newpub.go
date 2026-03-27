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

const PORT = 4444

var ALPNS = []string{"moq-00"} // Application Layer Protocols ["H3" - WebTransport]
const RELAY = "127.0.0.1:4444"

//const RELAY = "192.168.1.10:4443"

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
          Congestion: nil,
       },
       InsecureSkipVerify: true,
    }

    pub := api.NewMOQPub(Options, RELAY)
    log.Debug().Msgf("pub main before Connect %s", pub)
    handler, err := pub.Connect()
    log.Debug().Msgf("pub main after Connect %s", pub)
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
    //handler.SendAnnounceWithCacheTTL("bbb", 30)
    log.Debug().Msgf("pub main after SendAnnounce %s", pub)
    <-pub.Ctx.Done()
}

func handleStream(stream *moqt.PubStream) {
    stream.Accept()
    //time.After(time.Second * 2)
    groupid1 := uint64(0)
    for {
       //for range 3 {
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
          copy(payload1, []byte("gs1 "+timestamp)) // 将时间戳复制到 payload 前部
          // 填充剩余部分为固定内容（例如 "A"）
          for i := len(timestamp); i < length; i++ {
             payload1[i] = 'A'
          }

          gs1.WriteObject(&wire.Object{
             GroupID: groupid1,
             ID:      objectid,
             Payload: payload1,
          })
          objectid++
          time.Sleep(10 * time.Millisecond)
       }

       gs1.Close()
       groupid1 = groupid1 + 1

       //<-time.After(time.Millisecond * 1000)
    }
}