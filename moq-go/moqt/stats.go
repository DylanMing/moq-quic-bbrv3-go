package moqt

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

type ConnectionStatsProvider interface {
	GetConnectionStats() ConnectionStats
}

type ConnectionStats struct {
	MinRTT        time.Duration
	LatestRTT     time.Duration
	SmoothedRTT   time.Duration
	PacketsSent   uint64
	PacketsLost   uint64
}

type CongestionStatsProvider interface {
	GetCongestionStats() CongestionStats
}

type CongestionStats struct {
	CongestionWindow uint64
	PacingRate      uint64
	BytesInFlight   uint64
	TotalBytesSent  uint64
	TotalBytesLost uint64
	MaxBandwidth    uint64
	State           string
	InRecovery      bool
	InSlowStart    bool
	PacingGain     float64
	CwndGain       float64
}

type StatsLogger struct {
	provider ConnectionStatsProvider
	interval time.Duration
	stopChan chan struct{}
	logFunc func(stats ConnectionStats)
}

func NewStatsLogger(provider ConnectionStatsProvider, interval time.Duration) *StatsLogger {
	return &StatsLogger{
		provider: provider,
		interval: interval,
		stopChan: make(chan struct{}),
		logFunc:  defaultLogFunc,
	}
}

func NewStatsLoggerWithCallback(provider ConnectionStatsProvider, interval time.Duration, logFunc func(stats ConnectionStats)) *StatsLogger {
	return &StatsLogger{
		provider: provider,
		interval: interval,
		stopChan: make(chan struct{}),
		logFunc:  logFunc,
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
	if s.provider == nil {
		return
	}
	stats := s.provider.GetConnectionStats()
	s.logFunc(stats)
}

func defaultLogFunc(stats ConnectionStats) {
	log.Info().
		Str("MinRTT", stats.MinRTT.String()).
		Str("LatestRTT", stats.LatestRTT.String()).
		Str("SmoothedRTT", stats.SmoothedRTT.String()).
		Uint64("PacketsSent", stats.PacketsSent).
		Uint64("PacketsLost", stats.PacketsLost).
		Msg("[QUIC Stats]")
}

type CongestionStatsLogger struct {
	provider CongestionStatsProvider
	interval time.Duration
	stopChan chan struct{}
	logFunc  func(stats CongestionStats)
}

func NewCongestionStatsLogger(provider CongestionStatsProvider, interval time.Duration) *CongestionStatsLogger {
	return &CongestionStatsLogger{
		provider: provider,
		interval: interval,
		stopChan: make(chan struct{}),
		logFunc:  defaultCongestionLogFunc,
	}
}

func NewCongestionStatsLoggerWithCallback(provider CongestionStatsProvider, interval time.Duration, logFunc func(stats CongestionStats)) *CongestionStatsLogger {
	return &CongestionStatsLogger{
		provider: provider,
		interval: interval,
		stopChan: make(chan struct{}),
		logFunc:  logFunc,
	}
}

func (s *CongestionStatsLogger) Start() {
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

func (s *CongestionStatsLogger) Stop() {
	close(s.stopChan)
}

func (s *CongestionStatsLogger) logStats() {
	if s.provider == nil {
		return
	}
	stats := s.provider.GetCongestionStats()
	s.logFunc(stats)
}

func defaultCongestionLogFunc(stats CongestionStats) {
	log.Info().
		Str("CWND", FormatBytes(stats.CongestionWindow)).
		Str("PacingRate", FormatBandwidth(stats.PacingRate)).
		Str("BytesInFlight", FormatBytes(stats.BytesInFlight)).
		Str("TotalBytesSent", FormatBytes(stats.TotalBytesSent)).
		Str("TotalBytesLost", FormatBytes(stats.TotalBytesLost)).
		Str("MaxBandwidth", FormatBandwidth(stats.MaxBandwidth)).
		Str("State", stats.State).
		Bool("InRecovery", stats.InRecovery).
		Bool("InSlowStart", stats.InSlowStart).
		Float64("PacingGain", stats.PacingGain).
		Float64("CwndGain", stats.CwndGain).
		Msg("[Congestion Stats]")
}

func FormatBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(b)/1024)
	} else if b < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(b)/1024/1024)
	}
	return fmt.Sprintf("%.2f GB", float64(b)/1024/1024/1024)
}

func FormatBandwidth(bps uint64) string {
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
