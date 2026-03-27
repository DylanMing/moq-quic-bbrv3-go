package congestion

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type BBRv3StatsOptimized struct {
	enabled     atomic.Bool
	logInterval atomic.Int64
	lastLogTime atomic.Int64
	connID      atomic.Pointer[string]

	cwnd             atomic.Uint64
	bytesSent        atomic.Uint64
	bytesLost        atomic.Uint64
	minRtt           atomic.Int64
	avgRtt           atomic.Int64
	currentRtt       atomic.Int64
	rttCount         atomic.Uint64
	rttSum           atomic.Int64
	transmissionRate atomic.Uint64
	ssthresh         atomic.Uint64
	retransmitCount  atomic.Uint64

	state        atomic.Pointer[string]
	pacingRate   atomic.Uint64
	bandwidth    atomic.Uint64
	inflight     atomic.Uint64
	maxBandwidth atomic.Uint64

	logChan   chan BBRv3StatsSnapshot
	stopChan  chan struct{}
	waitGroup sync.WaitGroup

	logFunc func(snapshot BBRv3StatsSnapshot)
}

type BBRv3StatsConfigOptimized struct {
	Enabled     bool
	LogInterval time.Duration
	ConnID      string
	LogFunc     func(snapshot BBRv3StatsSnapshot)
}

func NewBBRv3StatsOptimized(config BBRv3StatsConfigOptimized) *BBRv3StatsOptimized {
	if config.LogInterval == 0 {
		config.LogInterval = time.Second
	}

	s := &BBRv3StatsOptimized{
		logChan:  make(chan BBRv3StatsSnapshot, 16),
		stopChan: make(chan struct{}),
	}

	s.enabled.Store(config.Enabled)
	s.logInterval.Store(int64(config.LogInterval))
	s.lastLogTime.Store(time.Now().UnixNano())

	if config.ConnID != "" {
		s.connID.Store(&config.ConnID)
	}

	if config.LogFunc != nil {
		s.logFunc = config.LogFunc
	} else {
		s.logFunc = defaultLogFunc
	}

	if config.Enabled {
		s.start()
	}

	return s
}

func defaultLogFunc(snapshot BBRv3StatsSnapshot) {
	fmt.Printf(`
========== BBRv3 Statistics ==========
Timestamp: %s
Connection ID: %s
--------------------------------------
State: %s
--------------------------------------
Congestion Window (CWND): %d bytes
Slow Start Threshold (SSTHRESH): %d bytes
--------------------------------------
Total Bytes Sent: %d bytes
Bytes Lost: %d bytes
Retransmission Count: %d
--------------------------------------
Min RTT: %v
Avg RTT: %v
Current RTT: %v
--------------------------------------
Transmission Rate: %d bytes/s (%.2f Mbps)
Bandwidth: %d bytes/s (%.2f Mbps)
Max Bandwidth: %d bytes/s (%.2f Mbps)
Pacing Rate: %d bytes/s
--------------------------------------
In-flight: %d bytes
======================================

`,
		time.Now().Format("2006-01-02 15:04:05.000"),
		snapshot.ConnID,
		snapshot.State,
		snapshot.CWND,
		snapshot.SSTHRESH,
		snapshot.BytesSent,
		snapshot.BytesLost,
		snapshot.RetransmitCount,
		snapshot.MinRTT,
		snapshot.AvgRTT,
		snapshot.CurrentRTT,
		snapshot.TransmissionRate, float64(snapshot.TransmissionRate)*8/1000000,
		snapshot.Bandwidth, float64(snapshot.Bandwidth)*8/1000000,
		snapshot.MaxBandwidth, float64(snapshot.MaxBandwidth)*8/1000000,
		snapshot.PacingRate,
		snapshot.Inflight,
	)
}

func (s *BBRv3StatsOptimized) start() {
	s.waitGroup.Add(1)
	go s.logWorker()
}

func (s *BBRv3StatsOptimized) logWorker() {
	defer s.waitGroup.Done()

	for {
		select {
		case snapshot := <-s.logChan:
			s.logFunc(snapshot)
		case <-s.stopChan:
			for {
				select {
				case snapshot := <-s.logChan:
					s.logFunc(snapshot)
				default:
					return
				}
			}
		}
	}
}

func (s *BBRv3StatsOptimized) Stop() {
	close(s.stopChan)
	s.waitGroup.Wait()
}

func (s *BBRv3StatsOptimized) SetEnabled(enabled bool) {
	wasEnabled := s.enabled.Load()
	s.enabled.Store(enabled)

	if enabled && !wasEnabled {
		s.stopChan = make(chan struct{})
		s.start()
	} else if !enabled && wasEnabled {
		close(s.stopChan)
		s.waitGroup.Wait()
	}
}

func (s *BBRv3StatsOptimized) SetLogInterval(interval time.Duration) {
	s.logInterval.Store(int64(interval))
}

func (s *BBRv3StatsOptimized) SetConnID(connID string) {
	s.connID.Store(&connID)
}

func (s *BBRv3StatsOptimized) SetLogFunc(logFunc func(snapshot BBRv3StatsSnapshot)) {
	s.logFunc = logFunc
}

func (s *BBRv3StatsOptimized) UpdateCWND(cwnd uint64) {
	s.cwnd.Store(cwnd)
}

func (s *BBRv3StatsOptimized) AddBytesSent(bytes uint64) {
	s.bytesSent.Add(bytes)
}

func (s *BBRv3StatsOptimized) AddBytesLost(bytes uint64) {
	s.bytesLost.Add(bytes)
}

func (s *BBRv3StatsOptimized) UpdateRTT(rtt time.Duration) {
	if rtt <= 0 {
		return
	}

	rttNanos := int64(rtt)

	currentMin := s.minRtt.Load()
	for rttNanos < currentMin && currentMin > 0 {
		if s.minRtt.CompareAndSwap(currentMin, rttNanos) {
			break
		}
		currentMin = s.minRtt.Load()
	}

	s.currentRtt.Store(rttNanos)

	s.rttSum.Add(rttNanos)
	s.rttCount.Add(1)

	count := s.rttCount.Load()
	if count > 0 {
		sum := s.rttSum.Load()
		s.avgRtt.Store(sum / int64(count))
	}
}

func (s *BBRv3StatsOptimized) UpdateTransmissionRate(rate uint64) {
	s.transmissionRate.Store(rate)
}

func (s *BBRv3StatsOptimized) UpdateSSThresh(ssthresh uint64) {
	s.ssthresh.Store(ssthresh)
}

func (s *BBRv3StatsOptimized) IncrementRetransmit() {
	s.retransmitCount.Add(1)
}

func (s *BBRv3StatsOptimized) UpdateState(state string) {
	s.state.Store(&state)
}

func (s *BBRv3StatsOptimized) UpdatePacingRate(rate uint64) {
	s.pacingRate.Store(rate)
}

func (s *BBRv3StatsOptimized) UpdateBandwidth(bw uint64) {
	s.bandwidth.Store(bw)
}

func (s *BBRv3StatsOptimized) UpdateInflight(inflight uint64) {
	s.inflight.Store(inflight)
}

func (s *BBRv3StatsOptimized) UpdateMaxBandwidth(maxBw uint64) {
	s.maxBandwidth.Store(maxBw)
}

func (s *BBRv3StatsOptimized) ShouldLog() bool {
	if !s.enabled.Load() {
		return false
	}

	now := time.Now().UnixNano()
	lastLog := s.lastLogTime.Load()
	interval := s.logInterval.Load()

	if now-lastLog >= interval {
		if s.lastLogTime.CompareAndSwap(lastLog, now) {
			return true
		}
	}
	return false
}

func (s *BBRv3StatsOptimized) Log() {
	if !s.enabled.Load() {
		return
	}

	snapshot := s.GetStatsSnapshot()

	select {
	case s.logChan <- snapshot:
	default:
	}
}

func (s *BBRv3StatsOptimized) GetStatsSnapshot() BBRv3StatsSnapshot {
	var connID, state string
	if ptr := s.connID.Load(); ptr != nil {
		connID = *ptr
	}
	if ptr := s.state.Load(); ptr != nil {
		state = *ptr
	}

	minRtt := s.minRtt.Load()
	avgRtt := s.avgRtt.Load()
	currentRtt := s.currentRtt.Load()

	return BBRv3StatsSnapshot{
		CWND:             s.cwnd.Load(),
		BytesSent:        s.bytesSent.Load(),
		BytesLost:        s.bytesLost.Load(),
		MinRTT:           time.Duration(minRtt),
		AvgRTT:           time.Duration(avgRtt),
		CurrentRTT:       time.Duration(currentRtt),
		TransmissionRate: s.transmissionRate.Load(),
		SSTHRESH:         s.ssthresh.Load(),
		RetransmitCount:  s.retransmitCount.Load(),
		State:            state,
		PacingRate:       s.pacingRate.Load(),
		Bandwidth:        s.bandwidth.Load(),
		Inflight:         s.inflight.Load(),
		MaxBandwidth:     s.maxBandwidth.Load(),
		ConnID:           connID,
	}
}
