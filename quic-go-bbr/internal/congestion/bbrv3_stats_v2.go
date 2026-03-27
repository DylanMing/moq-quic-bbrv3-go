package congestion

import (
	"encoding/json"
	"sync/atomic"
	"time"
	"unsafe"
)

type BBRv3StatsV2 struct {
	enabled     uint32
	logInterval int64
	lastLogTime int64

	data statsData

	logCallback unsafe.Pointer
	stopChan    chan struct{}
}

type statsData struct {
	cwnd             uint64
	bytesSent        uint64
	bytesLost        uint64
	minRtt           int64
	avgRtt           int64
	currentRtt       int64
	rttCount         uint64
	rttSum           int64
	transmissionRate uint64
	ssthresh         uint64
	retransmitCount  uint64
	state            uint32
	pacingRate       uint64
	bandwidth        uint64
	inflight         uint64
	maxBandwidth     uint64
	connIDHash       uint64
}

type BBRv3StatsConfigV2 struct {
	Enabled     bool
	LogInterval time.Duration
	ConnID      string
	LogCallback func(jsonData []byte)
}

var stateNames = [8]string{
	"Unknown",
	"Startup",
	"Drain",
	"ProbeBW-Down",
	"ProbeBW-Cruise",
	"ProbeBW-Refill",
	"ProbeBW-Up",
	"ProbeRTT",
}

func NewBBRv3StatsV2(config BBRv3StatsConfigV2) *BBRv3StatsV2 {
	if config.LogInterval == 0 {
		config.LogInterval = time.Second
	}

	s := &BBRv3StatsV2{
		logInterval: int64(config.LogInterval),
		lastLogTime: time.Now().UnixNano(),
		stopChan:    make(chan struct{}),
	}

	if config.Enabled {
		atomic.StoreUint32(&s.enabled, 1)
	}

	if config.ConnID != "" {
		s.data.connIDHash = hashString(config.ConnID)
	}

	if config.LogCallback != nil {
		s.SetLogCallback(config.LogCallback)
	}

	return s
}

func hashString(s string) uint64 {
	h := uint64(14695981039346656037)
	for _, c := range s {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

type StatsLogCallback func(jsonData []byte)

func (s *BBRv3StatsV2) SetLogCallback(cb StatsLogCallback) {
	atomic.StorePointer(&s.logCallback, unsafe.Pointer(&cb))
}

func (s *BBRv3StatsV2) getLogCallback() StatsLogCallback {
	ptr := atomic.LoadPointer(&s.logCallback)
	if ptr == nil {
		return nil
	}
	return *(*StatsLogCallback)(ptr)
}

func (s *BBRv3StatsV2) SetEnabled(enabled bool) {
	if enabled {
		atomic.StoreUint32(&s.enabled, 1)
	} else {
		atomic.StoreUint32(&s.enabled, 0)
	}
}

func (s *BBRv3StatsV2) SetLogInterval(interval time.Duration) {
	atomic.StoreInt64(&s.logInterval, int64(interval))
}

func (s *BBRv3StatsV2) SetConnID(connID string) {
	atomic.StoreUint64(&s.data.connIDHash, hashString(connID))
}

func (s *BBRv3StatsV2) UpdateCWND(cwnd uint64) {
	atomic.StoreUint64(&s.data.cwnd, cwnd)
}

func (s *BBRv3StatsV2) AddBytesSent(bytes uint64) {
	atomic.AddUint64(&s.data.bytesSent, bytes)
}

func (s *BBRv3StatsV2) AddBytesLost(bytes uint64) {
	atomic.AddUint64(&s.data.bytesLost, bytes)
}

func (s *BBRv3StatsV2) UpdateRTT(rtt time.Duration) {
	if rtt <= 0 {
		return
	}

	rttNanos := int64(rtt)

	for {
		current := atomic.LoadInt64(&s.data.minRtt)
		if current > 0 && rttNanos >= current {
			break
		}
		if atomic.CompareAndSwapInt64(&s.data.minRtt, current, rttNanos) {
			break
		}
	}

	atomic.StoreInt64(&s.data.currentRtt, rttNanos)
	atomic.AddInt64(&s.data.rttSum, rttNanos)
	count := atomic.AddUint64(&s.data.rttCount, 1)

	sum := atomic.LoadInt64(&s.data.rttSum)
	atomic.StoreInt64(&s.data.avgRtt, sum/int64(count))
}

func (s *BBRv3StatsV2) UpdateTransmissionRate(rate uint64) {
	atomic.StoreUint64(&s.data.transmissionRate, rate)
}

func (s *BBRv3StatsV2) UpdateSSThresh(ssthresh uint64) {
	atomic.StoreUint64(&s.data.ssthresh, ssthresh)
}

func (s *BBRv3StatsV2) IncrementRetransmit() {
	atomic.AddUint64(&s.data.retransmitCount, 1)
}

func (s *BBRv3StatsV2) UpdateState(state uint8) {
	atomic.StoreUint32(&s.data.state, uint32(state))
}

func (s *BBRv3StatsV2) UpdateStateByName(state string) {
	for i, name := range stateNames {
		if name == state {
			atomic.StoreUint32(&s.data.state, uint32(i))
			return
		}
	}
	atomic.StoreUint32(&s.data.state, 0)
}

func (s *BBRv3StatsV2) UpdatePacingRate(rate uint64) {
	atomic.StoreUint64(&s.data.pacingRate, rate)
}

func (s *BBRv3StatsV2) UpdateBandwidth(bw uint64) {
	atomic.StoreUint64(&s.data.bandwidth, bw)
}

func (s *BBRv3StatsV2) UpdateInflight(inflight uint64) {
	atomic.StoreUint64(&s.data.inflight, inflight)
}

func (s *BBRv3StatsV2) UpdateMaxBandwidth(maxBw uint64) {
	atomic.StoreUint64(&s.data.maxBandwidth, maxBw)
}

func (s *BBRv3StatsV2) ShouldLog() bool {
	if atomic.LoadUint32(&s.enabled) == 0 {
		return false
	}

	now := time.Now().UnixNano()
	lastLog := atomic.LoadInt64(&s.lastLogTime)
	interval := atomic.LoadInt64(&s.logInterval)

	if now-lastLog >= interval {
		return atomic.CompareAndSwapInt64(&s.lastLogTime, lastLog, now)
	}
	return false
}

type statsJSON struct {
	Timestamp        int64  `json:"ts"`
	ConnIDHash       uint64 `json:"conn_id_hash"`
	State            string `json:"state"`
	CWND             uint64 `json:"cwnd"`
	SSTHRESH         uint64 `json:"ssthresh"`
	BytesSent        uint64 `json:"bytes_sent"`
	BytesLost        uint64 `json:"bytes_lost"`
	RetransmitCount  uint64 `json:"retrans"`
	MinRTTUs         int64  `json:"min_rtt_us"`
	AvgRTTUs         int64  `json:"avg_rtt_us"`
	CurrentRTTUs     int64  `json:"cur_rtt_us"`
	TXRate           uint64 `json:"tx_rate"`
	Bandwidth        uint64 `json:"bw"`
	MaxBandwidth     uint64 `json:"max_bw"`
	PacingRate       uint64 `json:"pacing_rate"`
	Inflight         uint64 `json:"inflight"`
}

func (s *BBRv3StatsV2) Log() {
	if atomic.LoadUint32(&s.enabled) == 0 {
		return
	}

	cb := s.getLogCallback()
	if cb == nil {
		return
	}

	stateIdx := atomic.LoadUint32(&s.data.state)
	if stateIdx >= uint32(len(stateNames)) {
		stateIdx = 0
	}

	snapshot := statsJSON{
		Timestamp:       time.Now().UnixNano(),
		ConnIDHash:      atomic.LoadUint64(&s.data.connIDHash),
		State:           stateNames[stateIdx],
		CWND:            atomic.LoadUint64(&s.data.cwnd),
		SSTHRESH:        atomic.LoadUint64(&s.data.ssthresh),
		BytesSent:       atomic.LoadUint64(&s.data.bytesSent),
		BytesLost:       atomic.LoadUint64(&s.data.bytesLost),
		RetransmitCount: atomic.LoadUint64(&s.data.retransmitCount),
		MinRTTUs:        atomic.LoadInt64(&s.data.minRtt) / 1000,
		AvgRTTUs:        atomic.LoadInt64(&s.data.avgRtt) / 1000,
		CurrentRTTUs:    atomic.LoadInt64(&s.data.currentRtt) / 1000,
		TXRate:          atomic.LoadUint64(&s.data.transmissionRate),
		Bandwidth:       atomic.LoadUint64(&s.data.bandwidth),
		MaxBandwidth:    atomic.LoadUint64(&s.data.maxBandwidth),
		PacingRate:      atomic.LoadUint64(&s.data.pacingRate),
		Inflight:        atomic.LoadUint64(&s.data.inflight),
	}

	data, err := json.Marshal(&snapshot)
	if err != nil {
		return
	}

	cb(data)
}

func (s *BBRv3StatsV2) GetStatsSnapshot() BBRv3StatsSnapshot {
	stateIdx := atomic.LoadUint32(&s.data.state)
	if stateIdx >= uint32(len(stateNames)) {
		stateIdx = 0
	}

	return BBRv3StatsSnapshot{
		CWND:             atomic.LoadUint64(&s.data.cwnd),
		BytesSent:        atomic.LoadUint64(&s.data.bytesSent),
		BytesLost:        atomic.LoadUint64(&s.data.bytesLost),
		MinRTT:           time.Duration(atomic.LoadInt64(&s.data.minRtt)),
		AvgRTT:           time.Duration(atomic.LoadInt64(&s.data.avgRtt)),
		CurrentRTT:       time.Duration(atomic.LoadInt64(&s.data.currentRtt)),
		TransmissionRate: atomic.LoadUint64(&s.data.transmissionRate),
		SSTHRESH:         atomic.LoadUint64(&s.data.ssthresh),
		RetransmitCount:  atomic.LoadUint64(&s.data.retransmitCount),
		State:            stateNames[stateIdx],
		PacingRate:       atomic.LoadUint64(&s.data.pacingRate),
		Bandwidth:        atomic.LoadUint64(&s.data.bandwidth),
		Inflight:         atomic.LoadUint64(&s.data.inflight),
		MaxBandwidth:     atomic.LoadUint64(&s.data.maxBandwidth),
	}
}
