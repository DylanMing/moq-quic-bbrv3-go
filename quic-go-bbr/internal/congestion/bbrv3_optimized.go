package congestion

import (
	"math"
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
)

var _ SendAlgorithm = (*BBRv3SenderOptimized)(nil)
var _ SendAlgorithmWithDebugInfos = (*BBRv3SenderOptimized)(nil)

type bbrv3StateOptimized int8

const (
	bbrv3StateStartupOptimized bbrv3StateOptimized = iota
	bbrv3StateDrainOptimized
	bbrv3StateProbeBwDownOptimized
	bbrv3StateProbeBwCruiseOptimized
	bbrv3StateProbeBwRefillOptimized
	bbrv3StateProbeBwUpOptimized
	bbrv3StateProbeRttOptimized
)

type bbrv3ConfigOptimized struct {
	minCwnd              uint64
	initialCwnd          uint64
	initialRtt           time.Duration
	maxDatagramSize      uint64
	fullBwCountThreshold uint64
	fullBwGrowthRate     float64
	probeRttDuration     time.Duration
	probeRttInterval     time.Duration
	lossThreshold        float64
	fullLossCount        uint64
	beta                 float64
	headroom             float64
	minBdp               uint64
}

const (
	startupPacingGainOptimized          = 2.77
	pacingMarginPercentOptimized        = 0.01
	lossThreshOptimized                 = 0.02
	fullLossCntOptimized                = 6
	betaFactorOptimized                 = 0.7
	headroomFactorOptimized             = 0.85
	minPipeCwndInSmssOptimized          = 4
	extraAckedFilterLenOptimized        = 10
	minRttFilterLenOptimized            = 10 * time.Second
	probeRttDurationConstOptimized      = 200 * time.Millisecond
	probeRttIntervalConstOptimized      = 5 * time.Second
	fullBwCountThresholdConstOptimized  = 3
	fullBwGrowthRateConstOptimized      = 0.25
	probeBwMaxRoundsOptimized           = 63
	probeBwRandRoundsOptimized          = 2
	probeBwMinWaitTimeOptimized         = 2 * time.Second
	probeBwMaxWaitTimeOptimized         = 3 * time.Second
	sendQuantumThresholdPacingOptimized = 1_200_000 / 8
	defaultMaxDatagramSizeOptimized     = 1200
	defaultInitialCwndOptimized         = 10
	minBdpMultiplierOptimized           = 32
	minRttThresholdOptimized            = 100 * time.Microsecond
	bwFilterWindowOptimized             = 16
)

type roundTripCounterOptimized struct {
	roundCount         uint64
	isRoundStart       bool
	nextRoundDelivered uint64
}

type fullPipeEstimatorOptimized struct {
	isFilledPipe bool
	fullBw       uint64
	fullBwCount  uint64
}

type bandwidthFilterOptimized struct {
	bwWindow [bwFilterWindowOptimized]uint64
	idx      int
}

func newBandwidthFilterOptimized() *bandwidthFilterOptimized {
	return &bandwidthFilterOptimized{}
}

func (f *bandwidthFilterOptimized) update(bw uint64) {
	f.bwWindow[f.idx%bwFilterWindowOptimized] = bw
	f.idx++
}

func (f *bandwidthFilterOptimized) max() uint64 {
	maxBw := uint64(0)
	for i := 0; i < bwFilterWindowOptimized; i++ {
		if f.bwWindow[i] > maxBw {
			maxBw = f.bwWindow[i]
		}
	}
	return maxBw
}

type minMaxFilterOptimized struct {
	data   map[uint64]uint64
	idx    uint64
	window uint64
	curMax uint64
}

func newMinMaxFilterOptimized(window uint64) *minMaxFilterOptimized {
	return &minMaxFilterOptimized{
		data:   make(map[uint64]uint64),
		window: window,
	}
}

func (m *minMaxFilterOptimized) updateMax(time uint64, value uint64) {
	m.idx = time
	if value > m.curMax {
		m.curMax = value
	}
	m.data[time] = value

	for k := range m.data {
		if k+m.window < time {
			delete(m.data, k)
		}
	}
}

func (m *minMaxFilterOptimized) get() uint64 {
	maxVal := uint64(0)
	for _, v := range m.data {
		if v > maxVal {
			maxVal = v
		}
	}
	m.curMax = maxVal
	return m.curMax
}

type bbrv3AckInfoOptimized struct {
	ackedBytes protocol.ByteCount
	recordTime monotime.Time
	delivered  uint64
}

type BBRv3SenderOptimized struct {
	config *bbrv3ConfigOptimized
	state  bbrv3StateOptimized

	pacingRate  uint64
	pacingGain  float64
	sendQuantum uint64

	cwnd              uint64
	cwndGain          float64
	packetConservation bool
	priorCwnd         uint64

	round roundTripCounterOptimized

	idleRestart bool

	maxBw uint64
	bwHi  uint64
	bwLo  uint64
	bw    uint64

	minRtt           time.Duration
	minRttStamp      monotime.Time
	probeRttMinDelay time.Duration
	probeRttMinStamp monotime.Time
	probeRttExpired  bool

	bdpVal        uint64
	extraAcked    uint64
	offloadBudget uint64
	maxInflight   uint64
	inflightHi    uint64
	inflightLo    uint64

	bwLatest       uint64
	inflightLatest uint64

	maxBwFilter      *bandwidthFilterOptimized
	extraAckedFilter *minMaxFilterOptimized

	cycleCount uint64
	cycleStamp monotime.Time

	ackPhase int8

	extraAckedIntervalStart monotime.Time
	extraAckedDelivered     uint64

	fullPipe fullPipeEstimatorOptimized

	probeRttDoneStamp monotime.Time
	probeRttRoundDone bool

	roundsSinceBwProbe uint64
	bwProbeWait        time.Duration
	bwProbeUpCnt       uint64
	bwProbeUpAcks      uint64
	bwProbeUpRounds    uint64
	bwProbeSamples     bool

	lossRoundStart     bool
	lossInRound        bool
	inRecoveryMode     bool
	lossRoundDelivered uint64
	lossEventsInRound  uint64
	recoveryEpochStart monotime.Time

	sentTimes map[protocol.PacketNumber]monotime.Time

	delivered uint64
	ackinfo   []bbrv3AckInfoOptimized

	maxDatagramSize protocol.ByteCount

	nextSendTime monotime.Time

	lastNewMinRTT     time.Duration
	lastNewMinRTTTime monotime.Time
	lastProbeRttStart monotime.Time

	stats *BBRv3Stats

	totalBytesSent    uint64
	totalBytesLost    uint64
	retransmitCount   uint64

	bytesAckedThisRound uint64
}

func NewBBRv3SenderOptimized(initialMaxDatagramSize protocol.ByteCount) *BBRv3SenderOptimized {
	maxDatagramSize := uint64(initialMaxDatagramSize)
	if maxDatagramSize == 0 {
		maxDatagramSize = defaultMaxDatagramSizeOptimized
	}

	minBdp := minBdpMultiplierOptimized * maxDatagramSize

	config := &bbrv3ConfigOptimized{
		minCwnd:              minPipeCwndInSmssOptimized * maxDatagramSize,
		initialCwnd:          defaultInitialCwndOptimized * maxDatagramSize,
		initialRtt:           100 * time.Millisecond,
		maxDatagramSize:      maxDatagramSize,
		fullBwCountThreshold: fullBwCountThresholdConstOptimized,
		fullBwGrowthRate:     fullBwGrowthRateConstOptimized,
		probeRttDuration:     probeRttDurationConstOptimized,
		probeRttInterval:     probeRttIntervalConstOptimized,
		lossThreshold:        lossThreshOptimized,
		fullLossCount:        fullLossCntOptimized,
		beta:                 betaFactorOptimized,
		headroom:             headroomFactorOptimized,
		minBdp:               minBdp,
	}

	now := monotime.Now()

	s := &BBRv3SenderOptimized{
		config:  config,
		state:   bbrv3StateStartupOptimized,
		cwnd:    config.initialCwnd,
		pacingGain: startupPacingGainOptimized,
		cwndGain:   2.0,

		minRtt:           config.initialRtt,
		minRttStamp:      now,
		probeRttMinDelay: 10 * time.Second,
		probeRttMinStamp: now,

		maxBwFilter:      newBandwidthFilterOptimized(),
		extraAckedFilter: newMinMaxFilterOptimized(extraAckedFilterLenOptimized),

		bwHi:       math.MaxUint64,
		bwLo:       math.MaxUint64,
		inflightHi: math.MaxUint64,
		inflightLo: math.MaxUint64,

		extraAckedIntervalStart: now,

		sentTimes: make(map[protocol.PacketNumber]monotime.Time),

		maxDatagramSize: initialMaxDatagramSize,

		cycleStamp:        now,
		probeRttDoneStamp: now,
		bwProbeWait:       1 * time.Second,
		bwProbeUpCnt:      math.MaxUint64,

		recoveryEpochStart: now,

		lastNewMinRTT:     config.initialRtt,
		lastNewMinRTTTime: now,
		lastProbeRttStart: now,

		sendQuantum: maxDatagramSize,
	}

	s.initPacingRate()
	return s
}

func (b *BBRv3SenderOptimized) EnableStats(config BBRv3StatsConfig) {
	b.stats = NewBBRv3Stats(config)
}

func (b *BBRv3SenderOptimized) DisableStats() {
	if b.stats != nil {
		b.stats.SetEnabled(false)
	}
}

func (b *BBRv3SenderOptimized) GetStats() *BBRv3Stats {
	return b.stats
}

func (b *BBRv3SenderOptimized) initPacingRate() {
	nominalBandwidth := float64(b.config.initialCwnd) / b.config.initialRtt.Seconds()
	b.pacingRate = uint64(startupPacingGainOptimized * nominalBandwidth)
	b.bw = b.pacingRate
	b.maxBw = b.pacingRate
	b.maxBwFilter.update(b.maxBw)
}

func (b *BBRv3SenderOptimized) enterStartup() {
	b.state = bbrv3StateStartupOptimized
	b.pacingGain = startupPacingGainOptimized
	b.cwndGain = 2.0
}

func (b *BBRv3SenderOptimized) enterDrain() {
	b.state = bbrv3StateDrainOptimized
	b.pacingGain = 0.5
	b.cwndGain = 2.0
}

func (b *BBRv3SenderOptimized) enterProbeBw() {
	b.startProbeBwDown(monotime.Now())
}

func (b *BBRv3SenderOptimized) enterProbeRtt() {
	b.state = bbrv3StateProbeRttOptimized
	b.pacingGain = 1.0
	b.cwndGain = 1.0
	b.saveCwnd()
	b.probeRttDoneStamp = 0
	b.probeRttRoundDone = false
	b.ackPhase = 0
	b.startRound()
}

func (b *BBRv3SenderOptimized) startRound() {
	b.round.nextRoundDelivered = b.delivered
	b.bytesAckedThisRound = 0
}

func (b *BBRv3SenderOptimized) updateRound() {
	if b.delivered >= b.round.nextRoundDelivered && b.bytesAckedThisRound > 0 {
		b.startRound()
		b.round.roundCount++
		b.roundsSinceBwProbe++
		b.round.isRoundStart = true
		b.packetConservation = false
	} else {
		b.round.isRoundStart = false
	}
}

func (b *BBRv3SenderOptimized) isFilledPipe() bool {
	return b.fullPipe.isFilledPipe
}

func (b *BBRv3SenderOptimized) isInProbeBwState() bool {
	switch b.state {
	case bbrv3StateProbeBwDownOptimized, bbrv3StateProbeBwCruiseOptimized, 
	     bbrv3StateProbeBwRefillOptimized, bbrv3StateProbeBwUpOptimized:
		return true
	}
	return false
}

func (b *BBRv3SenderOptimized) checkStartupDone() {
	b.checkStartupFullBandwidth()
	b.checkStartupHighLoss()
	if b.state == bbrv3StateStartupOptimized && b.fullPipe.isFilledPipe {
		b.enterDrain()
	}
}

func (b *BBRv3SenderOptimized) checkStartupFullBandwidth() {
	if b.isFilledPipe() || !b.round.isRoundStart {
		return
	}

	if b.maxBw >= uint64(float64(b.fullPipe.fullBw)*(1.0+b.config.fullBwGrowthRate)) {
		b.fullPipe.fullBw = b.maxBw
		b.fullPipe.fullBwCount = 0
		return
	}

	b.fullPipe.fullBwCount++

	if b.fullPipe.fullBwCount >= b.config.fullBwCountThreshold {
		b.fullPipe.isFilledPipe = true
	}
}

func (b *BBRv3SenderOptimized) checkStartupHighLoss() {
}

func (b *BBRv3SenderOptimized) checkDrain(bytesInFlight uint64) {
	if b.state == bbrv3StateDrainOptimized && bytesInFlight <= b.inflight(1.0) {
		b.enterProbeBw()
	}
}

func (b *BBRv3SenderOptimized) startProbeBwDown(now monotime.Time) {
	b.bwProbeUpCnt = math.MaxUint64
	b.pickProbeWait()
	b.cycleStamp = now
	b.ackPhase = 1
	b.startRound()
	b.state = bbrv3StateProbeBwDownOptimized
	b.pacingGain = 0.9
	b.cwndGain = 2.0
}

func (b *BBRv3SenderOptimized) startProbeBwCruise() {
	b.state = bbrv3StateProbeBwCruiseOptimized
	b.pacingGain = 1.0
	b.cwndGain = 2.0
}

func (b *BBRv3SenderOptimized) startProbeBwRefill() {
	b.bwProbeUpRounds = 0
	b.bwProbeUpAcks = 0
	b.ackPhase = 2
	b.startRound()
	b.state = bbrv3StateProbeBwRefillOptimized
	b.pacingGain = 1.0
	b.cwndGain = 2.0
}

func (b *BBRv3SenderOptimized) startProbeBwUp(now monotime.Time) {
	b.ackPhase = 3
	b.startRound()
	b.fullPipe.fullBw = b.maxBw
	b.cycleStamp = now
	b.state = bbrv3StateProbeBwUpOptimized
	b.pacingGain = 1.25
	b.cwndGain = 2.25
	b.raiseInflightHiSlope()
}

func (b *BBRv3SenderOptimized) pickProbeWait() {
	b.roundsSinceBwProbe = 0
	b.bwProbeWait = 2500 * time.Millisecond
}

func (b *BBRv3SenderOptimized) hasElapsedInPhase(now monotime.Time, interval time.Duration) bool {
	return now.Sub(b.cycleStamp) >= interval
}

func (b *BBRv3SenderOptimized) checkTimeToProbeBw(now monotime.Time) bool {
	if b.hasElapsedInPhase(now, b.bwProbeWait) || b.isRenoCoexistenceProbeTime() {
		b.startProbeBwRefill()
		return true
	}
	return false
}

func (b *BBRv3SenderOptimized) isRenoCoexistenceProbeTime() bool {
	renoRounds := b.targetInflight()
	rounds := renoRounds
	if rounds > probeBwMaxRoundsOptimized {
		rounds = probeBwMaxRoundsOptimized
	}
	return b.roundsSinceBwProbe >= rounds
}

func (b *BBRv3SenderOptimized) checkTimeToCruise(bytesInFlight uint64) bool {
	if bytesInFlight > b.inflightWithHeadroom() {
		return false
	}
	if bytesInFlight <= b.inflight(1.0) {
		return true
	}
	return false
}

func (b *BBRv3SenderOptimized) raiseInflightHiSlope() {
	growthThisRound := uint64(1) << b.bwProbeUpRounds
	if b.bwProbeUpRounds < 30 {
		b.bwProbeUpRounds++
	}
	if growthThisRound > 0 {
		b.bwProbeUpCnt = b.cwnd / growthThisRound
		if b.bwProbeUpCnt < 1 {
			b.bwProbeUpCnt = 1
		}
	}
}

func (b *BBRv3SenderOptimized) probeInflightHiUpward(isCwndLimited bool) {
	if !isCwndLimited || b.cwnd < b.inflightHi {
		return
	}

	b.bwProbeUpAcks += uint64(b.config.maxDatagramSize)
	if b.bwProbeUpAcks >= b.bwProbeUpCnt {
		delta := b.bwProbeUpAcks / b.bwProbeUpCnt
		b.bwProbeUpAcks = b.bwProbeUpAcks % b.bwProbeUpCnt
		b.inflightHi = b.inflightHi + delta*b.config.maxDatagramSize
	}

	if b.round.isRoundStart {
		b.raiseInflightHiSlope()
	}
}

func (b *BBRv3SenderOptimized) updateProbeBwCyclePhase(now monotime.Time, bytesInFlight uint64) {
	if !b.isFilledPipe() {
		return
	}

	if !b.isInProbeBwState() {
		return
	}

	switch b.state {
	case bbrv3StateProbeBwDownOptimized:
		if b.checkTimeToProbeBw(now) {
			return
		}
		if b.checkTimeToCruise(bytesInFlight) {
			b.startProbeBwCruise()
		}
	case bbrv3StateProbeBwCruiseOptimized:
		b.checkTimeToProbeBw(now)
	case bbrv3StateProbeBwRefillOptimized:
		if b.round.isRoundStart {
			b.bwProbeSamples = true
			b.startProbeBwUp(now)
		}
	case bbrv3StateProbeBwUpOptimized:
		if b.hasElapsedInPhase(now, b.minRtt) && bytesInFlight > b.inflight(b.pacingGain) {
			b.startProbeBwDown(now)
		}
	}
}

func (b *BBRv3SenderOptimized) updateMinRtt(now monotime.Time) {
	elapsed := now.Sub(b.probeRttMinStamp)
	b.probeRttExpired = elapsed > b.config.probeRttInterval
}

func (b *BBRv3SenderOptimized) checkProbeRtt(now monotime.Time, bytesInFlight uint64) {
	if b.state != bbrv3StateProbeRttOptimized && b.probeRttExpired && !b.idleRestart {
		b.enterProbeRtt()
	}

	if b.state == bbrv3StateProbeRttOptimized {
		b.handleProbeRtt(now, bytesInFlight)
	}
}

func (b *BBRv3SenderOptimized) handleProbeRtt(now monotime.Time, bytesInFlight uint64) {
	probeRttCwnd := b.probeRttCwnd()
	
	if b.probeRttDoneStamp > 0 {
		if b.round.isRoundStart {
			b.probeRttRoundDone = true
		}
		if b.probeRttRoundDone {
			b.checkProbeRttDone(now)
		}
	} else if bytesInFlight <= probeRttCwnd {
		b.probeRttDoneStamp = now.Add(b.config.probeRttDuration)
		b.probeRttRoundDone = false
		b.startRound()
	}
}

func (b *BBRv3SenderOptimized) checkProbeRttDone(now monotime.Time) {
	if b.probeRttDoneStamp > 0 && now.Sub(b.probeRttDoneStamp) >= 0 {
		b.probeRttMinStamp = now
		b.restoreCwnd()
		b.exitProbeRtt(now)
	}
}

func (b *BBRv3SenderOptimized) probeRttCwnd() uint64 {
	probeRttCwnd := b.bdp(b.bw, 1.0)
	if probeRttCwnd < b.config.minCwnd {
		probeRttCwnd = b.config.minCwnd
	}
	return probeRttCwnd
}

func (b *BBRv3SenderOptimized) exitProbeRtt(now monotime.Time) {
	if b.isFilledPipe() {
		b.startProbeBwDown(now)
		b.startProbeBwCruise()
	} else {
		b.enterStartup()
	}
}

func (b *BBRv3SenderOptimized) saveCwnd() {
	if !b.inRecoveryMode && b.state != bbrv3StateProbeRttOptimized {
		b.priorCwnd = b.cwnd
	} else if b.cwnd > b.priorCwnd {
		b.priorCwnd = b.cwnd
	}
}

func (b *BBRv3SenderOptimized) restoreCwnd() {
	if b.cwnd < b.priorCwnd {
		b.cwnd = b.priorCwnd
	}
}

func (b *BBRv3SenderOptimized) enterRecovery(now monotime.Time) {
	b.saveCwnd()
	b.recoveryEpochStart = now
	b.cwnd = b.config.minCwnd + b.config.maxDatagramSize
	b.packetConservation = true
	b.inRecoveryMode = true
	b.startRound()
}

func (b *BBRv3SenderOptimized) exitRecovery() {
	b.recoveryEpochStart = 0
	b.packetConservation = false
	b.inRecoveryMode = false
	b.restoreCwnd()
}

func (b *BBRv3SenderOptimized) isInRecovery(sentTime monotime.Time) bool {
	return b.recoveryEpochStart > 0 && sentTime >= b.recoveryEpochStart
}

func (b *BBRv3SenderOptimized) bdp(bw uint64, gain float64) uint64 {
	if b.minRtt <= minRttThresholdOptimized {
		return max(b.config.minBdp, b.config.initialCwnd)
	}
	
	bdpVal := float64(bw) * b.minRtt.Seconds()
	result := uint64(gain * bdpVal)
	
	if result < b.config.minBdp {
		return b.config.minBdp
	}
	
	return result
}

func (b *BBRv3SenderOptimized) inflight(gain float64) uint64 {
	inflight := b.bdp(b.maxBw, gain)
	inflight = max(inflight, b.config.minCwnd)
	return inflight
}

func (b *BBRv3SenderOptimized) inflightWithHeadroom() uint64 {
	if b.inflightHi == math.MaxUint64 {
		return math.MaxUint64
	}
	headroom := uint64(float64(b.inflightHi) * (1.0 - b.config.headroom))
	if headroom < 1 {
		headroom = 1
	}
	if b.inflightHi > headroom {
		return max(b.inflightHi-headroom, b.config.minCwnd)
	}
	return b.config.minCwnd
}

func (b *BBRv3SenderOptimized) targetInflight() uint64 {
	return b.bdp(b.maxBw, b.cwndGain)
}

func (b *BBRv3SenderOptimized) quantizationBudget(inflight uint64) uint64 {
	b.offloadBudget = 3 * b.sendQuantum

	inflight = max(inflight, b.offloadBudget)
	inflight = max(inflight, b.config.minCwnd)

	if b.state == bbrv3StateProbeBwUpOptimized {
		inflight += 2 * b.config.maxDatagramSize
	}

	return inflight
}

func (b *BBRv3SenderOptimized) updateMaxInflight() {
	inflight := b.bdp(b.maxBw, b.cwndGain)
	inflight += b.extraAckedFilter.get()
	b.maxInflight = b.quantizationBudget(inflight)
}

func (b *BBRv3SenderOptimized) setPacingRate() {
	rate := uint64(b.pacingGain * float64(b.bw) * (1.0 - pacingMarginPercentOptimized))
	if b.isFilledPipe() || rate > b.pacingRate {
		b.pacingRate = rate
	}
}

func (b *BBRv3SenderOptimized) setSendQuantum() {
	var floor uint64
	if b.pacingRate < sendQuantumThresholdPacingOptimized {
		floor = b.config.maxDatagramSize
	} else {
		floor = 2 * b.config.maxDatagramSize
	}

	quantum := b.pacingRate / 1000
	if quantum < floor {
		quantum = floor
	}
	if quantum > 64*1024 {
		quantum = 64 * 1024
	}
	b.sendQuantum = quantum
}

func (b *BBRv3SenderOptimized) setCwnd(bytesInFlight uint64) {
	b.updateMaxInflight()

	if b.packetConservation {
		if b.cwnd < bytesInFlight+uint64(b.config.maxDatagramSize) {
			b.cwnd = bytesInFlight + uint64(b.config.maxDatagramSize)
		}
	} else {
		targetCwnd := b.maxInflight
		if targetCwnd > b.cwnd {
			b.cwnd = targetCwnd
		}
	}

	if b.cwnd < b.config.minCwnd {
		b.cwnd = b.config.minCwnd
	}

	b.boundCwndForProbeRtt()
	b.boundCwndForModel()
}

func (b *BBRv3SenderOptimized) boundCwndForProbeRtt() {
	if b.state == bbrv3StateProbeRttOptimized {
		limit := b.probeRttCwnd()
		if b.cwnd > limit {
			b.cwnd = limit
		}
	}
}

func (b *BBRv3SenderOptimized) boundCwndForModel() {
	var cap uint64 = math.MaxUint64

	if b.isInProbeBwState() && b.state != bbrv3StateProbeBwCruiseOptimized {
		cap = min(cap, b.inflightHi)
	} else if b.state == bbrv3StateProbeRttOptimized || b.state == bbrv3StateProbeBwCruiseOptimized {
		cap = min(cap, b.inflightWithHeadroom())
	}

	cap = min(cap, b.inflightLo)
	cap = max(cap, b.config.minCwnd)

	if b.cwnd > cap {
		b.cwnd = cap
	}
}

func (b *BBRv3SenderOptimized) updateAckAggregation(now monotime.Time) {
	if b.delivered < b.extraAckedDelivered {
		b.extraAckedDelivered = b.delivered
		b.extraAckedIntervalStart = now
		return
	}

	expectedDelivered := b.bw * uint64(now.Sub(b.extraAckedIntervalStart).Seconds())
	extraAckedNow := uint64(0)
	if b.delivered > b.extraAckedDelivered+expectedDelivered {
		extraAckedNow = b.delivered - b.extraAckedDelivered - expectedDelivered
	}

	b.extraAckedFilter.updateMax(uint64(now.Sub(monotime.Time(0))), extraAckedNow)
	b.extraAcked = b.extraAckedFilter.get()
}

func (b *BBRv3SenderOptimized) updateMaxBw(deliveryRate uint64) {
	if deliveryRate > 0 {
		b.maxBwFilter.update(deliveryRate)
		newMaxBw := b.maxBwFilter.max()
		if newMaxBw > b.maxBw {
			b.maxBw = newMaxBw
		}
		b.bw = deliveryRate
	}
}

func (b *BBRv3SenderOptimized) updateModelAndState(now monotime.Time, bytesInFlight uint64) {
	b.checkStartupDone()
	b.checkDrain(bytesInFlight)
	b.updateProbeBwCyclePhase(now, bytesInFlight)
	b.updateMinRtt(now)
	b.checkProbeRtt(now, bytesInFlight)
}

func (b *BBRv3SenderOptimized) updateControlParameters() {
	b.setPacingRate()
	b.setSendQuantum()
}

func (b *BBRv3SenderOptimized) TimeUntilSend(bytesInFlight protocol.ByteCount) monotime.Time {
	return b.nextSendTime
}

func (b *BBRv3SenderOptimized) HasPacingBudget(now monotime.Time) bool {
	if b.state == bbrv3StateProbeRttOptimized {
		return true
	}
	if b.pacingRate == 0 {
		return true
	}
	
	if b.pacingGain <= 1.0 {
		return true
	}
	
	return now >= b.nextSendTime
}

func (b *BBRv3SenderOptimized) OnPacketSent(sentTime monotime.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	b.sentTimes[packetNumber] = sentTime

	if isRetransmittable {
		b.totalBytesSent += uint64(bytes)
		if b.stats != nil {
			b.stats.AddBytesSent(uint64(bytes))
		}
	}

	if b.pacingRate > 0 {
		timePerByte := float64(time.Second) / (float64(b.pacingRate) * b.pacingGain)
		b.nextSendTime = sentTime.Add(time.Duration(float64(bytes) * timePerByte))
	}

	if bytesInFlight == 0 {
		b.idleRestart = true
	}
}

func (b *BBRv3SenderOptimized) CanSend(bytesInFlight protocol.ByteCount) bool {
	return uint64(bytesInFlight) < b.cwnd
}

func (b *BBRv3SenderOptimized) MaybeExitSlowStart() {
}

func (b *BBRv3SenderOptimized) OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime monotime.Time) {
	sentTime, ok := b.sentTimes[number]
	if !ok {
		return
	}
	delete(b.sentTimes, number)

	rtt := time.Duration(eventTime - sentTime)
	
	if rtt <= 0 {
		rtt = minRttThresholdOptimized
	}

	if b.stats != nil {
		b.stats.UpdateRTT(rtt)
	}

	if rtt > 0 && (b.lastNewMinRTT <= 0 || rtt < b.lastNewMinRTT) {
		b.lastNewMinRTT = rtt
		b.lastNewMinRTTTime = eventTime
		b.minRtt = rtt
		b.minRttStamp = eventTime
	}

	b.delivered += uint64(ackedBytes)
	b.bytesAckedThisRound += uint64(ackedBytes)
	
	b.ackinfo = append(b.ackinfo, bbrv3AckInfoOptimized{
		ackedBytes: ackedBytes,
		recordTime:  eventTime,
		delivered:   b.delivered,
	})

	if len(b.ackinfo) > 64 {
		b.ackinfo = b.ackinfo[len(b.ackinfo)-64:]
	}

	if b.state == bbrv3StateProbeRttOptimized {
		b.updateRound()
		b.maybeExitProbeRtt(eventTime)
		b.updateAndLogStats()
		return
	}

	b.updateRound()

	deliveryRate := b.calculateDeliveryRate(eventTime)
	b.updateMaxBw(deliveryRate)

	roundElapsed := eventTime.Sub(b.minRttStamp)
	if roundElapsed >= b.minRtt || b.round.isRoundStart {
		if b.inRecoveryMode {
			b.exitRecovery()
		}

		b.updateAckAggregation(eventTime)
		b.updateModelAndState(eventTime, uint64(priorInFlight))
		b.updateControlParameters()
		b.setCwnd(uint64(priorInFlight))

		if b.state == bbrv3StateDrainOptimized && priorInFlight < protocol.ByteCount(b.inflight(1.0)) {
			b.enterProbeBw()
		}

		b.minRttStamp = eventTime
		
		if eventTime.Sub(b.lastProbeRttStart) >= b.config.probeRttInterval {
			b.lastProbeRttStart = eventTime
			b.probeRttExpired = true
		}
	}

	b.updateAndLogStats()
}

func (b *BBRv3SenderOptimized) calculateDeliveryRate(eventTime monotime.Time) uint64 {
	if b.minRtt <= 0 || b.minRtt > 10*time.Second {
		return b.bw
	}
	
	var bytesInWindow uint64
	cutoff := eventTime.Add(-b.minRtt)
	
	for i := len(b.ackinfo) - 1; i >= 0; i-- {
		if b.ackinfo[i].recordTime < cutoff {
			break
		}
		bytesInWindow += uint64(b.ackinfo[i].ackedBytes)
	}
	
	if bytesInWindow == 0 {
		return b.bw
	}
	
	rate := uint64(float64(bytesInWindow) * float64(time.Second) / float64(b.minRtt))
	
	if rate > 0 {
		return rate
	}
	return b.bw
}

func (b *BBRv3SenderOptimized) maybeExitProbeRtt(now monotime.Time) {
	if b.state == bbrv3StateProbeRttOptimized && b.probeRttDoneStamp > 0 && 
	   now.Sub(b.probeRttDoneStamp) >= b.config.probeRttDuration {
		b.restoreCwnd()
		b.exitProbeRtt(now)
	}
}

func (b *BBRv3SenderOptimized) OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	b.totalBytesLost += uint64(lostBytes)
	b.retransmitCount++
	if b.stats != nil {
		b.stats.AddBytesLost(uint64(lostBytes))
		b.stats.IncrementRetransmit()
	}

	if !b.inRecoveryMode && b.state != bbrv3StateStartupOptimized && b.state != bbrv3StateProbeRttOptimized {
		b.enterRecovery(monotime.Now())
	}

	if b.cwnd > uint64(lostBytes) {
		b.cwnd -= uint64(lostBytes)
	} else {
		b.cwnd = b.config.minCwnd
	}
}

func (b *BBRv3SenderOptimized) OnRetransmissionTimeout(packetsRetransmitted bool) {
	if packetsRetransmitted {
		oldConfig := b.config
		oldMaxDatagramSize := b.maxDatagramSize
		*b = *NewBBRv3SenderOptimized(oldMaxDatagramSize)
		b.config = oldConfig
		b.maxDatagramSize = oldMaxDatagramSize
	}
}

func (b *BBRv3SenderOptimized) SetMaxDatagramSize(maxDatagramSize protocol.ByteCount) {
	b.maxDatagramSize = maxDatagramSize
	b.config.maxDatagramSize = uint64(maxDatagramSize)
	b.config.minCwnd = minPipeCwndInSmssOptimized * uint64(maxDatagramSize)
	b.config.minBdp = minBdpMultiplierOptimized * uint64(maxDatagramSize)
}

func (b *BBRv3SenderOptimized) GetCongestionWindow() protocol.ByteCount {
	return protocol.ByteCount(b.cwnd)
}

func (b *BBRv3SenderOptimized) InRecovery() bool {
	return b.inRecoveryMode
}

func (b *BBRv3SenderOptimized) InSlowStart() bool {
	return b.state == bbrv3StateStartupOptimized
}

func (b *BBRv3SenderOptimized) updateAndLogStats() {
	if b.stats == nil {
		return
	}

	stateStr := "Unknown"
	switch b.state {
	case bbrv3StateStartupOptimized:
		stateStr = "Startup"
	case bbrv3StateDrainOptimized:
		stateStr = "Drain"
	case bbrv3StateProbeBwDownOptimized:
		stateStr = "ProbeBW-Down"
	case bbrv3StateProbeBwCruiseOptimized:
		stateStr = "ProbeBW-Cruise"
	case bbrv3StateProbeBwRefillOptimized:
		stateStr = "ProbeBW-Refill"
	case bbrv3StateProbeBwUpOptimized:
		stateStr = "ProbeBW-Up"
	case bbrv3StateProbeRttOptimized:
		stateStr = "ProbeRTT"
	}

	b.stats.UpdateCWND(b.cwnd)
	b.stats.UpdateSSThresh(b.config.minCwnd)
	b.stats.UpdateTransmissionRate(b.pacingRate)
	b.stats.UpdateState(stateStr)
	b.stats.UpdatePacingRate(b.pacingRate)
	b.stats.UpdateBandwidth(b.bw)
	b.stats.UpdateMaxBandwidth(b.maxBw)

	if b.stats.ShouldLog() {
		b.stats.Log()
	}
}
