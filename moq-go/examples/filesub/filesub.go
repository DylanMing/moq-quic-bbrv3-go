package main

import (
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/DineshAdhi/moq-go/moqt"
	"github.com/DineshAdhi/moq-go/moqt/api"
	"github.com/DineshAdhi/moq-go/moqt/wire"
	"github.com/quic-go/quic-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const RELAY = "127.0.0.1:4444"

var ALPNS = []string{"moq-00"}

var outputFile string
var groupName string

func init() {
	flag.StringVar(&outputFile, "output", "received_file.bin", "Output file path")
	flag.StringVar(&groupName, "group", "filetest", "Group name to subscribe")
}

func main() {
	go func() {
		http.ListenAndServe(":8082", nil)
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

	Options := moqt.DialerOptions{
		ALPNs: ALPNS,
		QuicConfig: &quic.Config{
			KeepAlivePeriod: 1 * time.Second,
			EnableDatagrams: true,
			MaxIdleTimeout:  60 * time.Second,
			Congestion: func() quic.SendAlgorithmWithDebugInfos {
				return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "filesub"))
			},
		},
		InsecureSkipVerify: true,
	}

	sub := api.NewMOQSub(Options, RELAY)
	log.Info().Msgf("filesub: connecting to relay...")

	handler, err := sub.Connect()
	if err != nil {
		log.Error().Msgf("Error - %s", err)
		return
	}

	sub.OnStream(func(ss moqt.SubStream) {
		go handleFileStream(&ss)
	})

	sub.OnAnnounce(func(ns string) {
		log.Info().Msgf("Received announce: %s", ns)
		handler.Subscribe(ns, "filesub", 0)
	})

	log.Info().Msgf("filesub: subscribing to '%s'", groupName)
	handler.Subscribe(groupName, "filesub", 0)

	<-sub.Ctx.Done()
}

type GroupStats struct {
	GroupID     int
	Bytes       int64
	Objects     int
	StartTime   time.Time
	EndTime     time.Time
	FirstHeader string
}

var (
	groupStatsMap = make(map[int]*GroupStats)
	statsMutex    sync.Mutex
	totalBytes    int64
	totalObjects  int
	startTime     time.Time
)

func handleFileStream(ss *moqt.SubStream) {
	log.Info().Msgf("New Stream Header")
	startTime = time.Now()

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

		statsMutex.Lock()
		gid := int(groupid)
		if _, exists := groupStatsMap[gid]; !exists {
			groupStatsMap[gid] = &GroupStats{
				GroupID:   gid,
				StartTime: time.Now(),
			}
			if len(object.Payload) >= 30 {
				groupStatsMap[gid].FirstHeader = string(object.Payload[:30])
			}
		}
		groupStatsMap[gid].Bytes += int64(len(object.Payload))
		groupStatsMap[gid].Objects++
		groupStatsMap[gid].EndTime = time.Now()

		totalBytes += int64(len(object.Payload))
		totalObjects++
		statsMutex.Unlock()

		if object.ID == 0 || totalObjects%50 == 0 {
			header := ""
			if len(object.Payload) >= 30 {
				header = string(object.Payload[:30])
			}
			log.Info().Msgf("Received: Group=%d, Object=%d, Size=%d, Header=%s", groupid, object.ID, len(object.Payload), header)
		}
	}

	statsMutex.Lock()
	log.Info().Msgf("Stream ended. Total: %d bytes, %d objects", totalBytes, totalObjects)

	for gid, stats := range groupStatsMap {
		duration := stats.EndTime.Sub(stats.StartTime)
		throughput := float64(stats.Bytes) / 1024 / 1024 / duration.Seconds()
		log.Info().Msgf("Group %d stats: %d bytes, %d objects, %v duration, %.2f MB/s", 
			gid, stats.Bytes, stats.Objects, duration, throughput)
	}

	totalDuration := time.Since(startTime)
	avgThroughput := float64(totalBytes) / 1024 / 1024 / totalDuration.Seconds()
	log.Info().Msgf("=== TRANSFER COMPLETE ===")
	log.Info().Msgf("Total received: %d bytes (%.2f MB)", totalBytes, float64(totalBytes)/1024/1024)
	log.Info().Msgf("Total duration: %v", totalDuration)
	log.Info().Msgf("Average throughput: %.2f MB/s", avgThroughput)
	log.Info().Msgf("Total objects: %d", totalObjects)
	log.Info().Msgf("Total groups: %d", len(groupStatsMap))
	statsMutex.Unlock()
}
