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

type CubicStatsLogger struct {
	conn *quic.Conn
	interval time.Duration
	stopChan chan struct{}
}

func NewCubicStatsLogger(conn *quic.Conn, interval time.Duration) *CubicStatsLogger {
	return &CubicStatsLogger{
		conn: conn,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (s *CubicStatsLogger) Start() {
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

func (s *CubicStatsLogger) Stop() {
	close(s.stopChan)
}

func (s *CubicStatsLogger) logStats() {
	stats := s.conn.ConnectionStats()
	log.Info().
		Str("CWND", "N/A").
		Str("PacingRate", "N/A").
		Str("BytesInFlight", "N/A").
		Str("TotalBytesSent", formatBytes(uint64(stats.PacketsSent))).
		Str("TotalBytesLost", "N/A").
		Str("MinRTT", stats.MinRTT.String()).
		Str("SmoothedRTT", stats.SmoothedRTT.String()).
		Str("MaxBandwidth", "N/A").
		Float64("PacingGain", 0).
		Float64("CwndGain", 0).
		Str("State", "Cubic").
		Bool("InRecovery", false).
		Bool("InSlowStart", false).
		Msg("[Cubic Stats]")
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

	quicConfig := &quic.Config{
		KeepAlivePeriod: 1 * time.Second,
		EnableDatagrams: true,
		MaxIdleTimeout:  60 * time.Second,
	}

	tlsConfig := &tls.Config{
		NextProtos:         ALPNS,
		InsecureSkipVerify: true,
	}

	ctx := context.Background()
	log.Info().Msgf("Connecting to QUIC server at %s (using default Cubic)", RELAY)
	conn, err := quic.DialAddr(ctx, RELAY, tlsConfig, quicConfig)
	if err != nil {
		log.Error().Msgf("Failed to connect: %s", err)
		return
	}

	log.Info().Msgf("Connected successfully!")

	statsLogger := NewCubicStatsLogger(conn, STATS_INTERVAL)
	statsLogger.Start()

	time.Sleep(30 * time.Second)

	statsLogger.Stop()
	conn.CloseWithError(0, "normal close")
	log.Info().Msgf("Connection closed")
}
