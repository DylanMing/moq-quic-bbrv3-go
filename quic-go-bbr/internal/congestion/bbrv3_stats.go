package congestion

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type BBRv3Stats struct {
	mu sync.RWMutex

	enabled       bool
	logInterval   time.Duration
	lastLogTime   time.Time
	connID        string

	cwnd              uint64
	bytesSent         uint64
	bytesLost         uint64
	minRtt            time.Duration
	avgRtt            time.Duration
	currentRtt        time.Duration
	rttCount          uint64
	rttSum            time.Duration
	transmissionRate  uint64
	ssthresh          uint64
	retransmitCount   uint64

	state             string
	pacingRate        uint64
	bandwidth         uint64
	inflight          uint64
	maxBandwidth      uint64
}

type BBRv3StatsConfig struct {
	Enabled     bool
	LogInterval time.Duration
	ConnID      string
}

func NewBBRv3Stats(config BBRv3StatsConfig) *BBRv3Stats {
	if config.LogInterval == 0 {
		config.LogInterval = time.Second
	}
	return &BBRv3Stats{
		enabled:     config.Enabled,
		logInterval: config.LogInterval,
		connID:      config.ConnID,
		lastLogTime: time.Now(),
	}
}

func (s *BBRv3Stats) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

func (s *BBRv3Stats) SetLogInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logInterval = interval
}

func (s *BBRv3Stats) SetConnID(connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connID = connID
}

func (s *BBRv3Stats) UpdateCWND(cwnd uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cwnd = cwnd
}

func (s *BBRv3Stats) AddBytesSent(bytes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytesSent += bytes
}

func (s *BBRv3Stats) AddBytesLost(bytes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytesLost += bytes
}

func (s *BBRv3Stats) UpdateRTT(rtt time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if rtt > 0 {
		if s.minRtt == 0 || rtt < s.minRtt {
			s.minRtt = rtt
		}
		s.currentRtt = rtt
		s.rttSum += rtt
		s.rttCount++
		s.avgRtt = time.Duration(uint64(s.rttSum) / s.rttCount)
	}
}

func (s *BBRv3Stats) UpdateTransmissionRate(rate uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transmissionRate = rate
}

func (s *BBRv3Stats) UpdateSSThresh(ssthresh uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ssthresh = ssthresh
}

func (s *BBRv3Stats) IncrementRetransmit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retransmitCount++
}

func (s *BBRv3Stats) UpdateState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *BBRv3Stats) UpdatePacingRate(rate uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pacingRate = rate
}

func (s *BBRv3Stats) UpdateBandwidth(bw uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bandwidth = bw
}

func (s *BBRv3Stats) UpdateInflight(inflight uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inflight = inflight
}

func (s *BBRv3Stats) UpdateMaxBandwidth(maxBw uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxBandwidth = maxBw
}

func (s *BBRv3Stats) ShouldLog() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.enabled {
		return false
	}
	
	return time.Since(s.lastLogTime) >= s.logInterval
}

func (s *BBRv3Stats) Log() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.enabled {
		return
	}
	
	s.lastLogTime = time.Now()
	
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	
	log.Printf("\n========== BBRv3 Statistics ==========\n")
	log.Printf("Timestamp: %s\n", timestamp)
	if s.connID != "" {
		log.Printf("Connection ID: %s\n", s.connID)
	}
	log.Printf("--------------------------------------\n")
	log.Printf("State: %s\n", s.state)
	log.Printf("--------------------------------------\n")
	log.Printf("Congestion Window (CWND): %d bytes\n", s.cwnd)
	log.Printf("Slow Start Threshold (SSTHRESH): %d bytes\n", s.ssthresh)
	log.Printf("--------------------------------------\n")
	log.Printf("Total Bytes Sent: %d bytes\n", s.bytesSent)
	log.Printf("Bytes Lost: %d bytes\n", s.bytesLost)
	log.Printf("Retransmission Count: %d\n", s.retransmitCount)
	log.Printf("--------------------------------------\n")
	log.Printf("Min RTT: %v\n", s.minRtt)
	log.Printf("Avg RTT: %v\n", s.avgRtt)
	log.Printf("Current RTT: %v\n", s.currentRtt)
	log.Printf("--------------------------------------\n")
	log.Printf("Transmission Rate: %d bytes/s (%.2f Mbps)\n", s.transmissionRate, float64(s.transmissionRate)*8/1000000)
	log.Printf("Bandwidth: %d bytes/s (%.2f Mbps)\n", s.bandwidth, float64(s.bandwidth)*8/1000000)
	log.Printf("Max Bandwidth: %d bytes/s (%.2f Mbps)\n", s.maxBandwidth, float64(s.maxBandwidth)*8/1000000)
	log.Printf("Pacing Rate: %d bytes/s\n", s.pacingRate)
	log.Printf("--------------------------------------\n")
	log.Printf("In-flight: %d bytes\n", s.inflight)
	log.Printf("======================================\n\n")
}

func (s *BBRv3Stats) GetStatsSnapshot() BBRv3StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return BBRv3StatsSnapshot{
		CWND:             s.cwnd,
		BytesSent:        s.bytesSent,
		BytesLost:        s.bytesLost,
		MinRTT:           s.minRtt,
		AvgRTT:           s.avgRtt,
		CurrentRTT:       s.currentRtt,
		TransmissionRate: s.transmissionRate,
		SSTHRESH:         s.ssthresh,
		RetransmitCount:  s.retransmitCount,
		State:            s.state,
		PacingRate:       s.pacingRate,
		Bandwidth:        s.bandwidth,
		Inflight:         s.inflight,
		MaxBandwidth:     s.maxBandwidth,
	}
}

type BBRv3StatsSnapshot struct {
	CWND             uint64
	BytesSent        uint64
	BytesLost        uint64
	MinRTT           time.Duration
	AvgRTT           time.Duration
	CurrentRTT       time.Duration
	TransmissionRate uint64
	SSTHRESH         uint64
	RetransmitCount  uint64
	State            string
	PacingRate       uint64
	Bandwidth        uint64
	Inflight         uint64
	MaxBandwidth     uint64
	ConnID           string
}

func (s BBRv3StatsSnapshot) String() string {
	return fmt.Sprintf(
		"BBRv3Stats{CWND=%d, BytesSent=%d, BytesLost=%d, MinRTT=%v, AvgRTT=%v, CurrentRTT=%v, TXRate=%d, SSTHRESH=%d, Retrans=%d, State=%s}",
		s.CWND, s.BytesSent, s.BytesLost, s.MinRTT, s.AvgRTT, s.CurrentRTT, s.TransmissionRate, s.SSTHRESH, s.RetransmitCount, s.State,
	)
}
