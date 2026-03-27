package congestion

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type CongestionAlgorithm string

const (
	AlgorithmCUBIC CongestionAlgorithm = "CUBIC"
	AlgorithmBBRv1 CongestionAlgorithm = "BBRv1"
	AlgorithmBBRv3 CongestionAlgorithm = "BBRv3"
)

type StatsConfig struct {
	Enabled       bool
	LogInterval   time.Duration
	ConnID        string
	Algorithm     CongestionAlgorithm
	OutputWriter  io.Writer
	LogToStderr   bool
	Callback      func(snapshot StatsSnapshot)
	JSONOutput    bool
}

type StatsSnapshot struct {
	Timestamp        time.Time         `json:"ts"`
	ConnID           string            `json:"conn_id"`
	Algorithm        CongestionAlgorithm `json:"algo"`
	State            string            `json:"state"`
	CWND             uint64            `json:"cwnd"`
	SSTHRESH         uint64            `json:"ssthresh"`
	BytesSent        uint64            `json:"bytes_sent"`
	BytesLost        uint64            `json:"bytes_lost"`
	RetransmitCount  uint64            `json:"retrans"`
	MinRTT           time.Duration     `json:"min_rtt"`
	AvgRTT           time.Duration     `json:"avg_rtt"`
	CurrentRTT       time.Duration     `json:"cur_rtt"`
	TXRate           uint64            `json:"tx_rate"`
	Bandwidth        uint64            `json:"bw"`
	MaxBandwidth     uint64            `json:"max_bw"`
	PacingRate       uint64            `json:"pacing_rate"`
	Inflight         uint64            `json:"inflight"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

func (s StatsSnapshot) String() string {
	return fmt.Sprintf(
		"[%s] %s: CWND=%d, RTT(min=%v, avg=%v, cur=%v), BW=%d bytes/s, Sent=%d, Lost=%d, Retrans=%d, State=%s",
		s.Algorithm, s.ConnID, s.CWND, s.MinRTT, s.AvgRTT, s.CurrentRTT,
		s.Bandwidth, s.BytesSent, s.BytesLost, s.RetransmitCount, s.State,
	)
}

func (s StatsSnapshot) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

type StatsCollector interface {
	UpdateCWND(cwnd uint64)
	UpdateSSThresh(ssthresh uint64)
	AddBytesSent(bytes uint64)
	AddBytesLost(bytes uint64)
	IncrementRetransmit()
	UpdateRTT(rtt time.Duration)
	UpdateState(state string)
	UpdateTXRate(rate uint64)
	UpdateBandwidth(bw uint64)
	UpdateMaxBandwidth(maxBw uint64)
	UpdatePacingRate(rate uint64)
	UpdateInflight(inflight uint64)
	SetExtra(key string, value interface{})
	ShouldLog() bool
	Log()
	GetSnapshot() StatsSnapshot
	Enable()
	Disable()
	Stop()
}

type statsCollector struct {
	mu           sync.RWMutex
	enabled      atomic.Bool
	logInterval  time.Duration
	lastLogTime  atomic.Int64
	connID       string
	algorithm    CongestionAlgorithm

	cwnd            atomic.Uint64
	ssthresh        atomic.Uint64
	bytesSent       atomic.Uint64
	bytesLost       atomic.Uint64
	retransmitCount atomic.Uint64
	minRtt          atomic.Int64
	avgRtt          atomic.Int64
	currentRtt      atomic.Int64
	rttCount        atomic.Uint64
	rttSum          atomic.Int64
	txRate          atomic.Uint64
	bandwidth       atomic.Uint64
	maxBandwidth    atomic.Uint64
	pacingRate      atomic.Uint64
	inflight        atomic.Uint64

	state     atomic.Pointer[string]
	extra     atomic.Pointer[map[string]interface{}]

	outputWriter io.Writer
	logToStderr  bool
	callback     func(StatsSnapshot)
	jsonOutput   bool
}

func NewStatsCollector(config StatsConfig) StatsCollector {
	if config.LogInterval == 0 {
		config.LogInterval = time.Second
	}

	sc := &statsCollector{
		logInterval:  config.LogInterval,
		connID:       config.ConnID,
		algorithm:    config.Algorithm,
		outputWriter: config.OutputWriter,
		logToStderr:  config.LogToStderr,
		callback:     config.Callback,
		jsonOutput:   config.JSONOutput,
	}

	sc.enabled.Store(config.Enabled)
	sc.lastLogTime.Store(time.Now().UnixNano())

	if config.OutputWriter == nil && config.LogToStderr {
		sc.outputWriter = os.Stderr
	}

	return sc
}

func (s *statsCollector) UpdateCWND(cwnd uint64) {
	s.cwnd.Store(cwnd)
}

func (s *statsCollector) UpdateSSThresh(ssthresh uint64) {
	s.ssthresh.Store(ssthresh)
}

func (s *statsCollector) AddBytesSent(bytes uint64) {
	s.bytesSent.Add(bytes)
}

func (s *statsCollector) AddBytesLost(bytes uint64) {
	s.bytesLost.Add(bytes)
}

func (s *statsCollector) IncrementRetransmit() {
	s.retransmitCount.Add(1)
}

func (s *statsCollector) UpdateRTT(rtt time.Duration) {
	if rtt <= 0 {
		return
	}

	rttNanos := int64(rtt)

	for {
		current := s.minRtt.Load()
		if current > 0 && rttNanos >= current {
			break
		}
		if s.minRtt.CompareAndSwap(current, rttNanos) {
			break
		}
	}

	s.currentRtt.Store(rttNanos)
	s.rttSum.Add(rttNanos)
	count := s.rttCount.Add(1)

	sum := s.rttSum.Load()
	s.avgRtt.Store(sum / int64(count))
}

func (s *statsCollector) UpdateState(state string) {
	s.state.Store(&state)
}

func (s *statsCollector) UpdateTXRate(rate uint64) {
	s.txRate.Store(rate)
}

func (s *statsCollector) UpdateBandwidth(bw uint64) {
	s.bandwidth.Store(bw)
}

func (s *statsCollector) UpdateMaxBandwidth(maxBw uint64) {
	s.maxBandwidth.Store(maxBw)
}

func (s *statsCollector) UpdatePacingRate(rate uint64) {
	s.pacingRate.Store(rate)
}

func (s *statsCollector) UpdateInflight(inflight uint64) {
	s.inflight.Store(inflight)
}

func (s *statsCollector) SetExtra(key string, value interface{}) {
	for {
		oldPtr := s.extra.Load()
		var oldMap map[string]interface{}
		if oldPtr != nil {
			oldMap = *oldPtr
		}
		
		newMap := make(map[string]interface{})
		for k, v := range oldMap {
			newMap[k] = v
		}
		newMap[key] = value
		
		if s.extra.CompareAndSwap(oldPtr, &newMap) {
			break
		}
	}
}

func (s *statsCollector) ShouldLog() bool {
	if !s.enabled.Load() {
		return false
	}

	now := time.Now().UnixNano()
	lastLog := s.lastLogTime.Load()
	interval := int64(s.logInterval)

	if now-lastLog >= interval {
		return s.lastLogTime.CompareAndSwap(lastLog, now)
	}
	return false
}

func (s *statsCollector) Log() {
	if !s.enabled.Load() {
		return
	}

	snapshot := s.GetSnapshot()

	if s.callback != nil {
		s.callback(snapshot)
	}

	if s.jsonOutput {
		data, err := snapshot.ToJSON()
		if err != nil {
			return
		}
		if s.outputWriter != nil {
			s.outputWriter.Write(data)
			s.outputWriter.Write([]byte("\n"))
		}
	} else if s.outputWriter != nil {
		fmt.Fprintf(s.outputWriter, "%s\n", snapshot.String())
	}
}

func (s *statsCollector) GetSnapshot() StatsSnapshot {
	var state string
	if ptr := s.state.Load(); ptr != nil {
		state = *ptr
	}

	var extra map[string]interface{}
	if ptr := s.extra.Load(); ptr != nil {
		extra = *ptr
	}

	return StatsSnapshot{
		Timestamp:       time.Now(),
		ConnID:          s.connID,
		Algorithm:       s.algorithm,
		State:           state,
		CWND:            s.cwnd.Load(),
		SSTHRESH:        s.ssthresh.Load(),
		BytesSent:       s.bytesSent.Load(),
		BytesLost:       s.bytesLost.Load(),
		RetransmitCount: s.retransmitCount.Load(),
		MinRTT:          time.Duration(s.minRtt.Load()),
		AvgRTT:          time.Duration(s.avgRtt.Load()),
		CurrentRTT:      time.Duration(s.currentRtt.Load()),
		TXRate:          s.txRate.Load(),
		Bandwidth:       s.bandwidth.Load(),
		MaxBandwidth:    s.maxBandwidth.Load(),
		PacingRate:      s.pacingRate.Load(),
		Inflight:        s.inflight.Load(),
		Extra:           extra,
	}
}

func (s *statsCollector) Enable() {
	s.enabled.Store(true)
}

func (s *statsCollector) Disable() {
	s.enabled.Store(false)
}

func (s *statsCollector) Stop() {
	s.enabled.Store(false)
}

func DefaultStatsConfig(algorithm CongestionAlgorithm, connID string) StatsConfig {
	return StatsConfig{
		Enabled:      true,
		LogInterval:  time.Second,
		ConnID:       connID,
		Algorithm:    algorithm,
		LogToStderr:  true,
		JSONOutput:   false,
	}
}

func JSONStatsConfig(algorithm CongestionAlgorithm, connID string, callback func(StatsSnapshot)) StatsConfig {
	return StatsConfig{
		Enabled:      true,
		LogInterval:  time.Second,
		ConnID:       connID,
		Algorithm:    algorithm,
		Callback:     callback,
		JSONOutput:   true,
	}
}

func LogStatsToFile(config StatsConfig, filepath string) (StatsCollector, *os.File, error) {
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}
	config.OutputWriter = f
	return NewStatsCollector(config), f, nil
}

type StatsPrinter struct {
	collector StatsCollector
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

func NewStatsPrinter(collector StatsCollector, interval time.Duration) *StatsPrinter {
	sp := &StatsPrinter{
		collector: collector,
		stopChan:  make(chan struct{}),
	}
	sp.wg.Add(1)
	go sp.run(interval)
	return sp
}

func (sp *StatsPrinter) run(interval time.Duration) {
	defer sp.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sp.collector.Log()
		case <-sp.stopChan:
			return
		}
	}
}

func (sp *StatsPrinter) Stop() {
	close(sp.stopChan)
	sp.wg.Wait()
}

func PrintStatsHeader(w io.Writer) {
	fmt.Fprintln(w, "Timestamp | ConnID | Algorithm | State | CWND | MinRTT | AvgRTT | BW | BytesSent | BytesLost | Retrans")
}

func PrintStatsRow(w io.Writer, s StatsSnapshot) {
	fmt.Fprintf(w, "%s | %s | %s | %s | %d | %v | %v | %d | %d | %d | %d\n",
		s.Timestamp.Format("15:04:05.000"),
		s.ConnID,
		s.Algorithm,
		s.State,
		s.CWND,
		s.MinRTT,
		s.AvgRTT,
		s.Bandwidth,
		s.BytesSent,
		s.BytesLost,
		s.RetransmitCount,
	)
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}
