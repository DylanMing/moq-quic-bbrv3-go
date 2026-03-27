package congestion

import (
	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
)

type SendAlgorithmWithStats interface {
	SendAlgorithmWithDebugInfos
	GetStatsCollector() StatsCollector
}

type algorithmWithStats struct {
	inner     SendAlgorithmWithDebugInfos
	stats     StatsCollector
	algorithm CongestionAlgorithm
}

func WrapWithStats(inner SendAlgorithmWithDebugInfos, config StatsConfig) SendAlgorithmWithStats {
	if config.Algorithm == "" {
		config.Algorithm = AlgorithmCUBIC
	}
	return &algorithmWithStats{
		inner:     inner,
		stats:     NewStatsCollector(config),
		algorithm: config.Algorithm,
	}
}

func (a *algorithmWithStats) GetStatsCollector() StatsCollector {
	return a.stats
}

func (a *algorithmWithStats) TimeUntilSend(bytesInFlight protocol.ByteCount) monotime.Time {
	return a.inner.TimeUntilSend(bytesInFlight)
}

func (a *algorithmWithStats) HasPacingBudget(now monotime.Time) bool {
	return a.inner.HasPacingBudget(now)
}

func (a *algorithmWithStats) OnPacketSent(sentTime monotime.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	if isRetransmittable {
		a.stats.AddBytesSent(uint64(bytes))
	}
	a.inner.OnPacketSent(sentTime, bytesInFlight, packetNumber, bytes, isRetransmittable)
}

func (a *algorithmWithStats) CanSend(bytesInFlight protocol.ByteCount) bool {
	return a.inner.CanSend(bytesInFlight)
}

func (a *algorithmWithStats) MaybeExitSlowStart() {
	a.inner.MaybeExitSlowStart()
}

func (a *algorithmWithStats) OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime monotime.Time) {
	a.inner.OnPacketAcked(number, ackedBytes, priorInFlight, eventTime)

	a.stats.UpdateCWND(uint64(a.inner.GetCongestionWindow()))
	a.stats.UpdateInflight(uint64(priorInFlight))

	if a.algorithm == AlgorithmCUBIC {
		a.stats.UpdateState(a.getCubicState())
	} else {
		a.stats.UpdateState(a.getBBRState())
	}

	if a.stats.ShouldLog() {
		a.stats.Log()
	}
}

func (a *algorithmWithStats) OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	a.stats.AddBytesLost(uint64(lostBytes))
	a.stats.IncrementRetransmit()
	a.inner.OnCongestionEvent(number, lostBytes, priorInFlight)
}

func (a *algorithmWithStats) OnRetransmissionTimeout(packetsRetransmitted bool) {
	a.inner.OnRetransmissionTimeout(packetsRetransmitted)
}

func (a *algorithmWithStats) SetMaxDatagramSize(size protocol.ByteCount) {
	a.inner.SetMaxDatagramSize(size)
}

func (a *algorithmWithStats) InSlowStart() bool {
	return a.inner.InSlowStart()
}

func (a *algorithmWithStats) InRecovery() bool {
	return a.inner.InRecovery()
}

func (a *algorithmWithStats) GetCongestionWindow() protocol.ByteCount {
	cwnd := a.inner.GetCongestionWindow()
	a.stats.UpdateCWND(uint64(cwnd))
	return cwnd
}

func (a *algorithmWithStats) getCubicState() string {
	if a.inner.InSlowStart() {
		return "SlowStart"
	}
	if a.inner.InRecovery() {
		return "Recovery"
	}
	return "CongestionAvoidance"
}

func (a *algorithmWithStats) getBBRState() string {
	if a.inner.InSlowStart() {
		return "Startup"
	}
	if a.inner.InRecovery() {
		return "Recovery"
	}
	return "ProbeBW"
}

type BBRv1SenderWithStats struct {
	*BBRv1Sender
	stats StatsCollector
}

func NewBBRv1SenderWithStats(initialMaxDatagramSize protocol.ByteCount, config StatsConfig) *BBRv1SenderWithStats {
	config.Algorithm = AlgorithmBBRv1
	return &BBRv1SenderWithStats{
		BBRv1Sender: NewBBRv1Sender(initialMaxDatagramSize),
		stats:       NewStatsCollector(config),
	}
}

func (s *BBRv1SenderWithStats) GetStatsCollector() StatsCollector {
	return s.stats
}

func (s *BBRv1SenderWithStats) OnPacketSent(sentTime monotime.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	if isRetransmittable {
		s.stats.AddBytesSent(uint64(bytes))
	}
	s.BBRv1Sender.OnPacketSent(sentTime, bytesInFlight, packetNumber, bytes, isRetransmittable)
}

func (s *BBRv1SenderWithStats) OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime monotime.Time) {
	s.BBRv1Sender.OnPacketAcked(number, ackedBytes, priorInFlight, eventTime)

	s.stats.UpdateCWND(uint64(s.BBRv1Sender.GetCongestionWindow()))
	s.stats.UpdateInflight(uint64(priorInFlight))
	s.stats.UpdateState(s.getBBRv1State())

	if s.stats.ShouldLog() {
		s.stats.Log()
	}
}

func (s *BBRv1SenderWithStats) OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	s.stats.AddBytesLost(uint64(lostBytes))
	s.stats.IncrementRetransmit()
	s.BBRv1Sender.OnCongestionEvent(number, lostBytes, priorInFlight)
}

func (s *BBRv1SenderWithStats) getBBRv1State() string {
	if s.BBRv1Sender.InSlowStart() {
		return "Startup"
	}
	return "ProbeBW"
}

type BBRv3SenderWithStats struct {
	*BBRv3Sender
	stats StatsCollector
}

func NewBBRv3SenderWithStats(initialMaxDatagramSize protocol.ByteCount, config StatsConfig) *BBRv3SenderWithStats {
	config.Algorithm = AlgorithmBBRv3
	return &BBRv3SenderWithStats{
		BBRv3Sender: NewBBRv3Sender(initialMaxDatagramSize),
		stats:       NewStatsCollector(config),
	}
}

func (s *BBRv3SenderWithStats) GetStatsCollector() StatsCollector {
	return s.stats
}

func (s *BBRv3SenderWithStats) OnPacketSent(sentTime monotime.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	if isRetransmittable {
		s.stats.AddBytesSent(uint64(bytes))
	}
	s.BBRv3Sender.OnPacketSent(sentTime, bytesInFlight, packetNumber, bytes, isRetransmittable)
}

func (s *BBRv3SenderWithStats) OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime monotime.Time) {
	s.BBRv3Sender.OnPacketAcked(number, ackedBytes, priorInFlight, eventTime)

	s.stats.UpdateCWND(uint64(s.BBRv3Sender.GetCongestionWindow()))
	s.stats.UpdateInflight(uint64(priorInFlight))
	s.stats.UpdateState(s.getBBRv3State())

	if s.stats.ShouldLog() {
		s.stats.Log()
	}
}

func (s *BBRv3SenderWithStats) OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	s.stats.AddBytesLost(uint64(lostBytes))
	s.stats.IncrementRetransmit()
	s.BBRv3Sender.OnCongestionEvent(number, lostBytes, priorInFlight)
}

func (s *BBRv3SenderWithStats) getBBRv3State() string {
	if s.BBRv3Sender.InSlowStart() {
		return "Startup"
	}
	return "ProbeBW"
}
