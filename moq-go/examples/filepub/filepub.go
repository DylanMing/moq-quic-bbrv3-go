package main

import (
	"flag"
	"fmt"
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
const RELAY = "127.0.0.1:4444"
const FILE_SIZE = 10 * 1024 * 1024
const OBJECT_SIZE = 64 * 1024

var ALPNS = []string{"moq-00"}

var groupSize int
var groupName string
var algo string

func init() {
	flag.IntVar(&groupSize, "groupsize", 10*1024*1024, "Group size in bytes (default 10MB)")
	flag.StringVar(&groupName, "group", "filetest", "Group name for tracking")
	flag.StringVar(&algo, "algo", "bbrv3", "Congestion control algorithm: cubic, bbrv1, bbrv3")
}

func main() {
	go func() {
		http.ListenAndServe(":8081", nil)
	}()

	debug := flag.Bool("debug", false, "sets log level to debug")
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
	switch algo {
	case "cubic":
		congestionFunc = func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewCUBICWithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmCUBIC, "filepub"))
		}
	case "bbrv1":
		congestionFunc = func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "filepub"))
		}
	case "bbrv3":
		congestionFunc = func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewBBRv3OptimizedWithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "filepub"))
		}
	default:
		log.Error().Msgf("Unknown algorithm: %s, using bbrv3", algo)
		congestionFunc = func() quic.SendAlgorithmWithDebugInfos {
			return quic.NewBBRv3OptimizedWithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "filepub"))
		}
	}

	Options := moqt.DialerOptions{
		ALPNs: ALPNS,
		QuicConfig: &quic.Config{
			KeepAlivePeriod:                 1 * time.Second,
			EnableDatagrams:                 true,
			MaxIdleTimeout:                  60 * time.Second,
			InitialStreamReceiveWindow:      10 * 1024 * 1024,
			InitialConnectionReceiveWindow:  20 * 1024 * 1024,
			MaxStreamReceiveWindow:          10 * 1024 * 1024,
			MaxConnectionReceiveWindow:      20 * 1024 * 1024,
			Congestion:                      congestionFunc,
		},
		InsecureSkipVerify: true,
	}

	pub := api.NewMOQPub(Options, RELAY)
	log.Info().Msgf("filepub [%s]: connecting to relay...", algo)
	handler, err := pub.Connect()
	if err != nil {
		log.Error().Msgf("error - %s", err)
		return
	}

	pub.OnSubscribe(func(ps moqt.PubStream) {
		log.Info().Msgf("New Subscribe Request - %s", ps.TrackName)
		go handleFileStream(&ps)
	})

	log.Info().Msgf("filepub [%s]: sending announce for 'filetest'", algo)
	handler.SendAnnounce("filetest")

	numGroups := FILE_SIZE / groupSize
	if FILE_SIZE%groupSize != 0 {
		numGroups++
	}
	objectsPerGroup := groupSize / OBJECT_SIZE
	if groupSize%OBJECT_SIZE != 0 {
		objectsPerGroup++
	}

	log.Info().Msgf("Transfer config [algo=%s]: FileSize=%dMB, GroupSize=%.2fMB, Groups=%d, ObjectsPerGroup=%d, ObjectSize=%dKB",
		algo, FILE_SIZE/1024/1024, float64(groupSize)/1024/1024, numGroups, objectsPerGroup, OBJECT_SIZE/1024)

	<-pub.Ctx.Done()
}

func handleFileStream(stream *moqt.PubStream) {
	stream.Accept()

	startTime := time.Now()
	groupid := uint64(0)
	bytesSent := 0

	numGroups := FILE_SIZE / groupSize
	if FILE_SIZE%groupSize != 0 {
		numGroups++
	}

	for g := 0; g < numGroups; g++ {
		groupStart := time.Now()
		gs, err := stream.NewGroup(uint64(g))
		if err != nil {
			log.Error().Msgf("Err creating group - %s", err)
			return
		}

		groupBytes := 0
		groupLimit := groupSize
		remaining := FILE_SIZE - bytesSent
		if remaining < groupLimit {
			groupLimit = remaining
		}

		objectid := uint64(0)
		for groupBytes < groupLimit {
			objSize := OBJECT_SIZE
			if groupLimit-groupBytes < OBJECT_SIZE {
				objSize = groupLimit - groupBytes
			}

			payload := make([]byte, objSize)
			header := fmt.Sprintf("GROUP%d_OBJ%d_TIME%s ", g, objectid, time.Now().Format("15:04:05.000"))
			copy(payload, []byte(header))
			for i := len(header); i < objSize; i++ {
				payload[i] = byte((g + int(objectid)) % 256)
			}

			gs.WriteObject(&wire.Object{
				GroupID: uint64(g),
				ID:      objectid,
				Payload: payload,
			})

			time.Sleep(3 * time.Millisecond)

			bytesSent += objSize
			groupBytes += objSize
			objectid++
		}

		gs.Close()
		groupDuration := time.Since(groupStart)
		groupThroughput := float64(groupBytes) / 1024 / 1024 / groupDuration.Seconds()
		log.Info().Msgf("Group %d completed: %d bytes in %v (%.2f MB/s)", g, groupBytes, groupDuration, groupThroughput)
		groupid++
	}

	totalDuration := time.Since(startTime)
	avgThroughput := float64(bytesSent) / 1024 / 1024 / totalDuration.Seconds()
	log.Info().Msgf("Transfer complete [%s]: %d bytes in %v (%.2f MB/s avg)", algo, bytesSent, totalDuration, avgThroughput)
}
