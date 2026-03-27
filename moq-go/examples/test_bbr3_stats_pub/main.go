package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const RELAY = "127.0.0.1:4443"
const STATS_INTERVAL = 1 * time.Second

var ALPNS = []string{"moq-00"}

type StatsLogger struct {
	cong quic.SendAlgorithmWithDebugInfos
	interval time.Duration
	stopChan chan struct{}
}

func NewStatsLogger(cong quic.SendAlgorithmWithDebugInfos, interval time.Duration) *StatsLogger {
	return &StatsLogger{
		cong: cong,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (s *StatsLogger) Start() {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.logStats()
			case <-s.stopChan:
				return
			}
		}
	}()
}

func (s *StatsLogger) Stop() {
	close(s.stopChan)
}

func (s *StatsLogger) logStats() {
	stats := s.cong.GetStats(0)
	log.Info().
		Str("CWND", formatBytes(stats.CongestionWindow)).
		Str("PacingRate", formatBandwidth(stats.PacingRate)).
		Str("BytesInFlight", formatBytes(stats.BytesInFlight)).
		Str("TotalBytesSent", formatBytes(stats.TotalBytesSent)).
		Str("TotalBytesLost", formatBytes(stats.TotalBytesLost)).
		Str("MinRTT", stats.MinRTT.String()).
		Str("SmoothedRTT", stats.SmoothedRTT.String()).
		Str("MaxBandwidth", formatBandwidth(stats.MaxBandwidth)).
		Float64("PacingGain", stats.PacingGain).
		Float64("CwndGain", stats.CwndGain).
		Str("State", stats.State).
		Bool("InRecovery", stats.InRecovery).
		Bool("InSlowStart", stats.InSlowStart).
		Msg("[BBRv3 Stats]")
}

func formatBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(b)/1024)
	} else if b < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(b)/1024/1024)
	}
	return fmt.Sprintf("%.2f GB", float64(b)/1024/1024/1024)
}

func formatBandwidth(bps uint64) string {
	if bps == 0 {
		return "0 B/s"
	}
	bpsBytes := bps / 8
	if bpsBytes < 1024 {
		return fmt.Sprintf("%d B/s", bpsBytes)
	} else if bpsBytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB/s", float64(bpsBytes)/1024)
	} else if bpsBytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB/s", float64(bpsBytes)/1024/1024)
	}
	return fmt.Sprintf("%.2f GB/s", float64(bpsBytes)/1024/1024/1024)
}

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
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	var statsLogger *StatsLogger

	quicConfig := &quic.Config{
		KeepAlivePeriod: 1 * time.Second,
		EnableDatagrams: true,
		MaxIdleTimeout:  60 * time.Second,
		Congestion: func() quic.SendAlgorithmWithDebugInfos {
			cong := quic.NewBBRv3(nil)
			statsLogger = NewStatsLogger(cong, STATS_INTERVAL)
			return cong
		},
	}

	tlsConfig := &tls.Config{
		NextProtos:         ALPNS,
		InsecureSkipVerify: true,
	}

	ctx := context.Background()
	log.Info().Msgf("Connecting to QUIC server at %s", RELAY)
	conn, err := quic.DialAddr(ctx, RELAY, tlsConfig, quicConfig)
	if err != nil {
		log.Error().Msgf("Failed to connect: %s", err)
		return
	}

	log.Info().Msgf("Connected successfully!")

	statsLogger.Start()

	time.Sleep(30 * time.Second)

	statsLogger.Stop()
	conn.CloseWithError(0, "normal close")
	log.Info().Msgf("Connection closed")
}
