package congestion

import (
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
)

var _ SendAlgorithm = (*BBRv3Sender)(nil)
var _ SendAlgorithmWithDebugInfos = (*BBRv3Sender)(nil)

// BBRv3 State Machine
type bbrv3State int8

const (
	bbrv3StateStartup bbrv3State = iota
	bbrv3StateDrain
	bbrv3StateProbeBwDown
	bbrv3StateProbeBwCruise
	bbrv3StateProbeBwRefill
	bbrv3StateProbeBwUp
	bbrv3StateProbeRTT
)

// Ack probe phase
type ackProbePhase int8

const (
	ackProbePhaseInit ackProbePhase = iota
	ackProbePhaseStopping
	ackProbePhaseRefilling
	ackProbePhaseStarting
	ackProbePhaseFeedback
)

// BBRv3 configurable parameters
type bbrv3Config struct {
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
}

// BBRv3 constants
const (
	startupPacingGain          = 2.77
	pacingMarginPercent        = 0.01
	lossThresh                 = 0.02
	fullLossCnt                = 6
	betaFactor                 = 0.7
	headroomFactor             = 0.85
	minPipeCwndInSmss          = 4
	extraAckedFilterLen        = 10
	minRttFilterLen            = 10 * time.Second
	probeRttDurationConst      = 200 * time.Millisecond
	probeRttIntervalConst      = 5 * time.Second
	fullBwCountThresholdConst  = 3
	fullBwGrowthRateConst      = 0.25
	probeBwMaxRounds           = 63
	probeBwRandRounds          = 2
	probeBwMinWaitTime         = 2 * time.Second
	probeBwMaxWaitTime         = 3 * time.Second
	sendQuantumThresholdPacing = 1_200_000 / 8
	defaultMaxDatagramSize     = 1200
	defaultInitialCwnd         = 10
)

// Round trip counter
type roundTripCounter struct {
	roundCount          uint64
	isRoundStart        bool
	nextRoundDelivered  uint64
}

// Full pipe estimator
type fullPipeEstimator struct {
	isFilledPipe bool
	fullBw       uint64
	fullBwCount  uint64
}

// Simple max filter for bandwidth
type simpleMaxFilter struct {
	bw [2]uint64
}

func newSimpleMaxFilter() *simpleMaxFilter {
	return &simpleMaxFilter{}
}

func (f *simpleMaxFilter) maxBw() uint64 {
	if f.bw[0] > f.bw[1] {
		return f.bw[0]
	}
	return f.bw[1]
}

// MinMax filter for extra acked
type minMaxFilter struct {
	data   map[uint64]uint64
	idx    uint64
	window uint64
	curMax uint64
}

func newMinMaxFilter(window uint64) *minMaxFilter {
	return &minMaxFilter{
		data:   make(map[uint64]uint64),
		window: window,
	}
}

func (m *minMaxFilter) updateMax(time uint64, value uint64) {
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

func (m *minMaxFilter) get() uint64 {
	maxVal := uint64(0)
	for _, v := range m.data {
		if v > maxVal {
			maxVal = v
		}
	}
	m.curMax = maxVal
	return m.curMax
}

// ackInfo stores ACK information
type bbrv3AckInfo struct {
	ackedBytes protocol.ByteCount
	recordTime monotime.Time
	delivered  uint64
}

// BBRv3 sender
type BBRv3Sender struct {
	config *bbrv3Config
	state  bbrv3State

	// Pacing
	pacingRate  uint64
	pacingGain  float64
	sendQuantum uint64

	// Congestion window
	cwnd              uint64
	cwndGain          float64
	packetConservation bool
	priorCwnd         uint64

	// Round trip counter
	round roundTripCounter

	// Idle restart
	idleRestart bool

	// Bandwidth
	maxBw uint64
	bwHi  uint64
	bwLo  uint64
	bw    uint64

	// RTT
	minRtt           time.Duration
	minRttStamp      monotime.Time
	probeRttMinDelay time.Duration
	probeRttMinStamp monotime.Time
	probeRttExpired  bool

	// BDP and inflight
	bdpVal         uint64
	extraAcked     uint64
	offloadBudget  uint64
	maxInflight    uint64
	inflightHi     uint64
	inflightLo     uint64

	// Latest signals
	bwLatest       uint64
	inflightLatest uint64

	// Filters
	maxBwFilter      *simpleMaxFilter
	extraAckedFilter *minMaxFilter

	// Cycle
	cycleCount uint64
	cycleStamp monotime.Time

	// ACK probing
	ackPhase ackProbePhase

	// Extra acked
	extraAckedIntervalStart monotime.Time
	extraAckedDelivered     uint64

	// Full pipe
	fullPipe fullPipeEstimator

	// Probe RTT
	probeRttDoneStamp monotime.Time
	probeRttRoundDone bool

	// BW probe
	roundsSinceBwProbe uint64
	bwProbeWait        time.Duration
	bwProbeUpCnt      uint64
	bwProbeUpAcks     uint64
	bwProbeUpRounds   uint64
	bwProbeSamples    bool

	// Loss
	lossRoundStart     bool
	lossInRound        bool
	inRecoveryMode     bool
	lossRoundDelivered uint64
	lossEventsInRound  uint64
	recoveryEpochStart monotime.Time

	// Packet tracking
	sentTimes map[protocol.PacketNumber]monotime.Time

	// Delivery tracking
	delivered uint64
	ackinfo   []bbrv3AckInfo

	// Max datagram size
	maxDatagramSize protocol.ByteCount

	// Runtime state
	nextSendTime monotime.Time

	// Last min RTT tracking (from bbrv1)
	lastNewMinRTT     time.Duration
	lastNewMinRTTTime monotime.Time

	// Stats tracking
	totalBytesSent uint64
	totalBytesLost uint64
	lastRTT        time.Duration
	smoothedRTT    time.Duration
}

// NewBBRv3Sender creates a new BBRv3 sender
func NewBBRv3Sender(initialMaxDatagramSize protocol.ByteCount) *BBRv3Sender {
	maxDatagramSize := uint64(initialMaxDatagramSize)
	if maxDatagramSize == 0 {
		maxDatagramSize = defaultMaxDatagramSize
	}

	config := &bbrv3Config{
		minCwnd:              minPipeCwndInSmss * maxDatagramSize,
		initialCwnd:          defaultInitialCwnd * maxDatagramSize,
		initialRtt:           100 * time.Millisecond,
		maxDatagramSize:      maxDatagramSize,
		fullBwCountThreshold: fullBwCountThresholdConst,
		fullBwGrowthRate:     fullBwGrowthRateConst,
		probeRttDuration:     probeRttDurationConst,
		probeRttInterval:     probeRttIntervalConst,
		lossThreshold:        lossThresh,
		fullLossCount:        fullLossCnt,
		beta:                 betaFactor,
		headroom:             headroomFactor,
	}

	now := monotime.Now()

	s := &BBRv3Sender{
		config:  config,
		state:   bbrv3StateStartup,
		cwnd:    config.initialCwnd,
		pacingGain: 2.77,
		cwndGain:   2.0,

		minRtt:           10 * time.Second,
		minRttStamp:      now,
		probeRttMinDelay: 10 * time.Second,
		probeRttMinStamp: now,

		maxBwFilter:      newSimpleMaxFilter(),
		extraAckedFilter: newMinMaxFilter(extraAckedFilterLen),

		bwHi:       ^uint64(0),
		inflightHi: ^uint64(0),

		extraAckedIntervalStart: now,

		sentTimes: make(map[protocol.PacketNumber]monotime.Time),

		maxDatagramSize: initialMaxDatagramSize,

		cycleStamp:        now,
		probeRttDoneStamp: now,
		bwProbeWait:       1 * time.Second,
		bwProbeUpCnt:      ^uint64(0),

		recoveryEpochStart: now,

		lastNewMinRTT:     10 * time.Second,
		lastNewMinRTTTime: now,
	}

	s.init()
	return s
}

func (b *BBRv3Sender) init() {
	now := monotime.Now()

	b.minRtt = b.config.initialRtt
	if b.minRtt == 0 {
		b.minRtt = time.Microsecond
	}
	b.minRttStamp = now
	b.probeRttDoneStamp = 0
	b.probeRttRoundDone = false
	b.priorCwnd = 0
	b.idleRestart = false
	b.extraAckedIntervalStart = now
	b.extraAckedDelivered = 0
	b.ackPhase = ackProbePhaseInit
	b.bwHi = ^uint64(0)
	b.inflightHi = ^uint64(0)

	b.resetCongestionSignals()
	b.resetLowerBounds()
	b.initRoundCounting()
	b.initFullPipe()
	b.initPacingRate()

	b.enterStartup()
}

// Init round counting
func (b *BBRv3Sender) initRoundCounting() {
	b.round.nextRoundDelivered = b.delivered
	b.round.roundCount = 0
	b.round.isRoundStart = false
}

func (b *BBRv3Sender) startRound() {
	b.round.nextRoundDelivered = b.delivered
}

func (b *BBRv3Sender) updateRound() {
	maxDelivered := uint64(0)
	for _, info := range b.ackinfo {
		if info.delivered > maxDelivered {
			maxDelivered = info.delivered
		}
	}

	if maxDelivered >= b.round.nextRoundDelivered {
		b.startRound()
		b.round.roundCount++
		b.roundsSinceBwProbe++
		b.round.isRoundStart = true
		b.packetConservation = false
	} else {
		b.round.isRoundStart = false
	}
}

func (b *BBRv3Sender) initFullPipe() {
	b.fullPipe.isFilledPipe = false
	b.fullPipe.fullBw = 0
	b.fullPipe.fullBwCount = 0
}

func (b *BBRv3Sender) initPacingRate() {
	nominalBandwidth := float64(b.config.initialCwnd) / b.config.initialRtt.Seconds()
	b.pacingRate = uint64(startupPacingGain * nominalBandwidth)
}

func (b *BBRv3Sender) enterStartup() {
	b.state = bbrv3StateStartup
	b.updateGains()
}

func (b *BBRv3Sender) enterDrain() {
	b.state = bbrv3StateDrain
	b.updateGains()
}

func (b *BBRv3Sender) enterProbeBw() {
	b.startProbeBwDown(monotime.Now())
}

func (b *BBRv3Sender) enterProbeRtt() {
	b.state = bbrv3StateProbeRTT
	b.updateGains()
}

func (b *BBRv3Sender) updateGains() {
	switch b.state {
	case bbrv3StateStartup:
		b.pacingGain = 2.77
		b.cwndGain = 2.0
	case bbrv3StateDrain:
		b.pacingGain = 0.5
		b.cwndGain = 2.0
	case bbrv3StateProbeBwDown:
		b.pacingGain = 0.9
		b.cwndGain = 2.0
	case bbrv3StateProbeBwCruise:
		b.pacingGain = 1.0
		b.cwndGain = 2.0
	case bbrv3StateProbeBwRefill:
		b.pacingGain = 1.0
		b.cwndGain = 2.0
	case bbrv3StateProbeBwUp:
		b.pacingGain = 1.25
		b.cwndGain = 2.25
	case bbrv3StateProbeRTT:
		b.pacingGain = 1.0
		b.cwndGain = 0.5
	}
}

func (b *BBRv3Sender) resetCongestionSignals() {
	b.lossInRound = false
	b.bwLatest = 0
	b.inflightLatest = 0
}

func (b *BBRv3Sender) resetLowerBounds() {
	b.bwLo = ^uint64(0)
	b.inflightLo = ^uint64(0)
}

func (b *BBRv3Sender) initLowerBounds() {
	if b.bwLo == ^uint64(0) {
		b.bwLo = b.maxBw
	}
	if b.inflightLo == ^uint64(0) {
		b.inflightLo = b.cwnd
	}
}

func (b *BBRv3Sender) lossLowerBounds() {
	if b.bwLatest > b.bwLo {
		b.bwLo = b.bwLatest
	} else {
		b.bwLo = uint64(float64(b.bwLo) * b.config.beta)
	}
	if b.inflightLatest > b.inflightLo {
		b.inflightLo = b.inflightLatest
	} else {
		b.inflightLo = uint64(float64(b.inflightLo) * b.config.beta)
	}
}

func (b *BBRv3Sender) boundBwForModel() {
	b.bw = b.maxBw
	if b.bwLo < b.bw {
		b.bw = b.bwLo
	}
	if b.bwHi < b.bw {
		b.bw = b.bwHi
	}
}

// BDP calculation
func (b *BBRv3Sender) bdp(bw uint64, gain float64) uint64 {
	if b.minRtt == 0 {
		return b.config.initialCwnd
	}
	bdp := float64(bw) * b.minRtt.Seconds()
	b.bdpVal = uint64(bdp)
	return uint64(gain * bdp)
}

// inflight calculation with quantization budget
func (b *BBRv3Sender) quantizationBudget(inflight uint64) uint64 {
	b.offloadBudget = 3 * b.sendQuantum

	inflight = max(inflight, b.offloadBudget)
	inflight = max(inflight, b.config.minCwnd)

	if b.state == bbrv3StateProbeBwUp {
		inflight += 2 * b.config.maxDatagramSize
	}

	return inflight
}

func (b *BBRv3Sender) inflight(gain float64) uint64 {
	inflight := b.bdp(b.maxBw, gain)
	return b.quantizationBudget(inflight)
}

func (b *BBRv3Sender) inflightWithHeadroom() uint64 {
	if b.inflightHi == ^uint64(0) {
		return ^uint64(0)
	}
	headroom := uint64(float64(b.inflightHi) * b.config.headroom)
	if headroom < 1 {
		headroom = 1
	}
	if b.inflightHi > headroom {
		return max(b.inflightHi-headroom, b.config.minCwnd)
	}
	return b.config.minCwnd
}

func (b *BBRv3Sender) targetInflight() uint64 {
	if b.bdpVal < b.cwnd {
		return b.bdpVal
	}
	return b.cwnd
}

func (b *BBRv3Sender) isFilledPipe() bool {
	return b.fullPipe.isFilledPipe
}

func (b *BBRv3Sender) isInProbeBwState() bool {
	switch b.state {
	case bbrv3StateProbeBwDown, bbrv3StateProbeBwCruise, bbrv3StateProbeBwRefill, bbrv3StateProbeBwUp:
		return true
	}
	return false
}

func (b *BBRv3Sender) isProbingBw() bool {
	switch b.state {
	case bbrv3StateStartup, bbrv3StateProbeBwRefill, bbrv3StateProbeBwUp:
		return true
	}
	return false
}

// Check startup done
func (b *BBRv3Sender) checkStartupDone() {
	b.checkStartupFullBandwidth()
	b.checkStartupHighLoss()
	if b.state == bbrv3StateStartup && b.fullPipe.isFilledPipe {
		b.enterDrain()
	}
}

func (b *BBRv3Sender) checkStartupFullBandwidth() {
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

func (b *BBRv3Sender) checkStartupHighLoss() {
	// Simplified
}

func (b *BBRv3Sender) checkDrain(bytesInFlight uint64) {
	if b.state == bbrv3StateDrain && bytesInFlight <= b.inflight(1.0) {
		b.enterProbeBw()
	}
}

// ProbeBW methods
func (b *BBRv3Sender) startProbeBwDown(now monotime.Time) {
	b.resetCongestionSignals()
	b.bwProbeUpCnt = ^uint64(0)
	b.pickProbeWait()
	b.cycleStamp = now
	b.ackPhase = ackProbePhaseStopping
	b.startRound()
	b.state = bbrv3StateProbeBwDown
}

func (b *BBRv3Sender) startProbeBwCruise() {
	b.state = bbrv3StateProbeBwCruise
}

func (b *BBRv3Sender) startProbeBwRefill() {
	b.resetLowerBounds()
	b.bwProbeUpRounds = 0
	b.bwProbeUpAcks = 0
	b.ackPhase = ackProbePhaseRefilling
	b.startRound()
	b.state = bbrv3StateProbeBwRefill
}

func (b *BBRv3Sender) startProbeBwUp(now monotime.Time) {
	b.ackPhase = ackProbePhaseStarting
	b.startRound()
	b.fullPipe.fullBw = b.maxBw
	b.cycleStamp = now
	b.state = bbrv3StateProbeBwUp
	b.raiseInflightHiSlope()
}

func (b *BBRv3Sender) pickProbeWait() {
	b.roundsSinceBwProbe = 0
	b.bwProbeWait = 2500 * time.Millisecond
}

func (b *BBRv3Sender) hasElapsedInPhase(now monotime.Time, interval time.Duration) bool {
	return now.Sub(b.cycleStamp) >= interval
}

func (b *BBRv3Sender) checkTimeToProbeBw(now monotime.Time) bool {
	if b.hasElapsedInPhase(now, b.bwProbeWait) || b.isRenoCoexistenceProbeTime() {
		b.startProbeBwRefill()
		return true
	}
	return false
}

func (b *BBRv3Sender) isRenoCoexistenceProbeTime() bool {
	renoRounds := b.targetInflight()
	rounds := renoRounds
	if rounds > probeBwMaxRounds {
		rounds = probeBwMaxRounds
	}
	return b.roundsSinceBwProbe >= rounds
}

func (b *BBRv3Sender) checkTimeToCruise(bytesInFlight uint64) bool {
	if bytesInFlight > b.inflightWithHeadroom() {
		return false
	}
	if bytesInFlight <= b.inflight(1.0) {
		return true
	}
	return false
}

func (b *BBRv3Sender) raiseInflightHiSlope() {
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

func (b *BBRv3Sender) probeInflightHiUpward(isCwndLimited bool) {
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

func (b *BBRv3Sender) adaptUpperBounds() {
	if b.ackPhase == ackProbePhaseStarting && b.round.isRoundStart {
		b.ackPhase = ackProbePhaseFeedback
	}

	if b.ackPhase == ackProbePhaseStopping && b.round.isRoundStart {
		b.bwProbeSamples = false
		b.ackPhase = ackProbePhaseInit

		if b.isInProbeBwState() && b.maxBw > 0 {
			b.advanceMaxBwFilter()
		}
	}

	if !b.checkInflightTooHigh() {
		if b.inflightHi == ^uint64(0) {
			return
		}
	}

	if b.state == bbrv3StateProbeBwUp {
		b.probeInflightHiUpward(true)
	}
}

func (b *BBRv3Sender) checkInflightTooHigh() bool {
	return false
}

func (b *BBRv3Sender) advanceMaxBwFilter() {
	b.cycleCount++
	if b.maxBwFilter.bw[1] == 0 {
		return
	}
	b.maxBwFilter.bw[0] = b.maxBwFilter.bw[1]
	b.maxBwFilter.bw[1] = 0
}

func (b *BBRv3Sender) updateProbeBwCyclePhase(now monotime.Time, bytesInFlight uint64) {
	if !b.isFilledPipe() {
		return
	}

	b.adaptUpperBounds()

	if !b.isInProbeBwState() {
		return
	}

	switch b.state {
	case bbrv3StateProbeBwDown:
		if b.checkTimeToProbeBw(now) {
			return
		}
		if b.checkTimeToCruise(bytesInFlight) {
			b.startProbeBwCruise()
		}
	case bbrv3StateProbeBwCruise:
		b.checkTimeToProbeBw(now)
	case bbrv3StateProbeBwRefill:
		if b.round.isRoundStart {
			b.bwProbeSamples = true
			b.startProbeBwUp(now)
		}
	case bbrv3StateProbeBwUp:
		if b.hasElapsedInPhase(now, b.minRtt) && bytesInFlight > b.inflight(b.pacingGain) {
			b.startProbeBwDown(now)
		}
	}
}

// ProbeRTT methods
func (b *BBRv3Sender) updateMinRtt(now monotime.Time) {
	elapsed := now.Sub(b.probeRttMinStamp)
	b.probeRttExpired = elapsed > b.config.probeRttInterval

	if b.minRtt == 0 || b.probeRttExpired {
		b.minRtt = b.config.initialRtt
		b.minRttStamp = now
	}
}

func (b *BBRv3Sender) checkProbeRtt(now monotime.Time, bytesInFlight uint64) {
	if b.state != bbrv3StateProbeRTT && b.probeRttExpired && !b.idleRestart {
		b.enterProbeRtt()
		b.saveCwnd()
		b.probeRttDoneStamp = 0
		b.ackPhase = ackProbePhaseStopping
		b.startRound()
	}

	if b.state == bbrv3StateProbeRTT {
		b.handleProbeRtt(now, bytesInFlight)
	}
}

func (b *BBRv3Sender) handleProbeRtt(now monotime.Time, bytesInFlight uint64) {
	if b.probeRttDoneStamp > 0 {
		if b.round.isRoundStart {
			b.probeRttRoundDone = true
		}
		if b.probeRttRoundDone {
			b.checkProbeRttDone(now)
		}
	} else if bytesInFlight <= b.probeRttCwnd() {
		b.probeRttDoneStamp = now.Add(b.config.probeRttDuration)
		b.probeRttRoundDone = false
		b.startRound()
	}
}

func (b *BBRv3Sender) checkProbeRttDone(now monotime.Time) {
	if b.probeRttDoneStamp > 0 && now.Sub(b.probeRttDoneStamp) >= 0 {
		b.probeRttMinStamp = now
		b.restoreCwnd()
		b.exitProbeRtt(now)
	}
}

func (b *BBRv3Sender) probeRttCwnd() uint64 {
	return b.bdp(b.bw, b.cwndGain)
}

func (b *BBRv3Sender) exitProbeRtt(now monotime.Time) {
	b.resetLowerBounds()
	if b.isFilledPipe() {
		b.startProbeBwDown(now)
		b.startProbeBwCruise()
	} else {
		b.enterStartup()
	}
}

func (b *BBRv3Sender) saveCwnd() {
	if !b.inRecoveryMode && b.state != bbrv3StateProbeRTT {
		b.priorCwnd = b.cwnd
	} else if b.cwnd > b.priorCwnd {
		b.priorCwnd = b.cwnd
	}
}

func (b *BBRv3Sender) restoreCwnd() {
	if b.cwnd < b.priorCwnd {
		b.cwnd = b.priorCwnd
	}
}

// Recovery methods
func (b *BBRv3Sender) enterRecovery(now monotime.Time) {
	b.saveCwnd()
	b.recoveryEpochStart = now
	b.cwnd = b.config.minCwnd + b.config.maxDatagramSize
	b.packetConservation = true
	b.inRecoveryMode = true
	b.startRound()
}

func (b *BBRv3Sender) exitRecovery() {
	b.recoveryEpochStart = 0
	b.packetConservation = false
	b.inRecoveryMode = false
	b.restoreCwnd()
}

func (b *BBRv3Sender) isInRecovery(sentTime monotime.Time) bool {
	return b.recoveryEpochStart > 0 && sentTime >= b.recoveryEpochStart
}

// Control parameter updates
func (b *BBRv3Sender) setPacingRate() {
	b.setPacingRateWithGain(b.pacingGain)
}

func (b *BBRv3Sender) setPacingRateWithGain(gain float64) {
	rate := uint64(gain * float64(b.bw) * (1.0 - pacingMarginPercent))
	if b.isFilledPipe() || rate > b.pacingRate {
		b.pacingRate = rate
	}
}

func (b *BBRv3Sender) setSendQuantum() {
	var floor uint64
	if b.pacingRate < sendQuantumThresholdPacing {
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

func (b *BBRv3Sender) setCwnd(bytesInFlight uint64) {
	b.updateMaxInflight()
	b.modulateCwndForRecovery(bytesInFlight)

	if !b.packetConservation {
		if b.isFilledPipe() {
			if b.cwnd < b.maxInflight {
				// cwnd += newly_acked_bytes
			}
		} else if b.cwnd < b.maxInflight {
			// cwnd += newly_acked_bytes
		}
		if b.cwnd < b.config.minCwnd {
			b.cwnd = b.config.minCwnd
		}
	}

	b.boundCwndForProbeRtt()
	b.boundCwndForModel()
}

func (b *BBRv3Sender) updateMaxInflight() {
	inflight := b.bdp(b.maxBw, b.cwndGain)
	inflight += b.extraAckedFilter.get()
	b.maxInflight = b.quantizationBudget(inflight)
}

func (b *BBRv3Sender) modulateCwndForRecovery(bytesInFlight uint64) {
	if b.packetConservation {
		if b.cwnd < bytesInFlight+uint64(b.config.maxDatagramSize) {
			b.cwnd = bytesInFlight + uint64(b.config.maxDatagramSize)
		}
	}
}

func (b *BBRv3Sender) boundCwndForProbeRtt() {
	if b.state == bbrv3StateProbeRTT {
		limit := b.probeRttCwnd()
		if b.cwnd > limit {
			b.cwnd = limit
		}
	}
}

func (b *BBRv3Sender) boundCwndForModel() {
	var cap uint64 = ^uint64(0)

	if b.isInProbeBwState() && b.state != bbrv3StateProbeBwCruise {
		cap = min(cap, b.inflightHi)
	} else if b.state == bbrv3StateProbeRTT || b.state == bbrv3StateProbeBwCruise {
		cap = min(cap, b.inflightWithHeadroom())
	}

	cap = min(cap, b.inflightLo)
	cap = max(cap, b.config.minCwnd)

	if b.cwnd > cap {
		b.cwnd = cap
	}
}

func (b *BBRv3Sender) updateModelAndState(now monotime.Time, bytesInFlight uint64) {
	b.updateLatestDeliverySignals()
	b.updateCongestionSignals()
	b.updateAckAggregation(now)
	b.checkStartupDone()
	b.checkDrain(bytesInFlight)
	b.updateProbeBwCyclePhase(now, bytesInFlight)
	b.updateMinRtt(now)
	b.checkProbeRtt(now, bytesInFlight)
	b.advanceLatestDeliverySignals()
	b.boundBwForModel()
}

func (b *BBRv3Sender) updateControlParameters() {
	b.setPacingRate()
	b.setSendQuantum()
}

func (b *BBRv3Sender) updateLatestDeliverySignals() {
	b.lossRoundStart = false
}

func (b *BBRv3Sender) advanceLatestDeliverySignals() {
	if b.lossRoundStart {
	}
}

func (b *BBRv3Sender) updateCongestionSignals() {
	b.updateMaxBw()
}

func (b *BBRv3Sender) updateMaxBw() {
	// Simplified
}

func (b *BBRv3Sender) updateAckAggregation(now monotime.Time) {
	b.extraAcked = 0
}

// SendAlgorithm interface implementation

// TimeUntilSend returns when the next packet can be sent
func (b *BBRv3Sender) TimeUntilSend(bytesInFlight protocol.ByteCount) monotime.Time {
	return b.nextSendTime
}

// HasPacingBudget returns whether we have budget to send more packets
func (b *BBRv3Sender) HasPacingBudget(now monotime.Time) bool {
	b.maybeExitProbeRTT(now)
	if b.state == bbrv3StateProbeRTT {
		return true
	}
	return true
}

func (b *BBRv3Sender) maybeExitProbeRTT(now monotime.Time) {
	if b.state == bbrv3StateProbeRTT && b.probeRttDoneStamp > 0 && now.Sub(b.probeRttDoneStamp) >= b.config.probeRttDuration {
		b.exitProbeRTTFromAbove(now)
	}
}

func (b *BBRv3Sender) exitProbeRTTFromAbove(now monotime.Time) {
	if b.isFilledPipe() {
		b.startProbeBwDown(now)
		b.startProbeBwCruise()
	} else {
		b.enterStartup()
	}
}

// OnPacketSent is called when a packet is sent
func (b *BBRv3Sender) OnPacketSent(sentTime monotime.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	b.sentTimes[packetNumber] = sentTime
	b.totalBytesSent += uint64(bytes)

	if b.pacingRate > 0 {
		timePerByte := float64(time.Second) / (float64(b.pacingRate) * b.pacingGain)
		b.nextSendTime = sentTime.Add(time.Duration(float64(bytes) * timePerByte))
	}

	if bytesInFlight == 0 && b.idleRestart {
		// Pace at exactly bw
	}
}

// CanSend returns whether we can send more data
func (b *BBRv3Sender) CanSend(bytesInFlight protocol.ByteCount) bool {
	return uint64(bytesInFlight) < b.cwnd
}

// MaybeExitSlowStart is called to check if we should exit slow start
func (b *BBRv3Sender) MaybeExitSlowStart() {
	// BBRv3 uses its own startup detection
}

// OnPacketAcked is called when a packet is acknowledged
func (b *BBRv3Sender) OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime monotime.Time) {
	sentTime, ok := b.sentTimes[number]
	if !ok {
		return
	}
	delete(b.sentTimes, number)

	rtt := time.Duration(eventTime - sentTime)

	// Update min RTT (from bbrv1 style)
	if rtt > 0 && (b.lastNewMinRTT <= 0 || rtt < b.lastNewMinRTT) {
		b.lastNewMinRTT = rtt
		b.lastNewMinRTTTime = eventTime
		b.minRtt = rtt
		b.minRttStamp = eventTime
	}

	// Update delivered bytes and ackinfo
	b.delivered += uint64(ackedBytes)
	b.ackinfo = append(b.ackinfo, bbrv3AckInfo{
		ackedBytes: ackedBytes,
		recordTime:  eventTime,
		delivered:   b.delivered,
	})

	// Keep ackinfo window small
	if len(b.ackinfo) > 16 {
		b.ackinfo = b.ackinfo[len(b.ackinfo)-16:]
	}

	// May exit ProbeRTT
	b.maybeExitProbeRTT(eventTime)

	if b.state == bbrv3StateProbeRTT {
		return
	}

	// Update round
	b.updateRound()

	// Update delivery rate
	deliveryRate := b.calculateDeliveryRate(eventTime)
	if deliveryRate > b.maxBw || (b.bw > 0 && deliveryRate > 0) {
		b.maxBw = deliveryRate
		b.maxBwFilter.bw[1] = b.maxBw
	}

	// Check if round trip has passed
	roundElapsed := eventTime.Sub(b.minRttStamp)
	if roundElapsed >= b.minRtt || b.round.isRoundStart {
		if b.inRecoveryMode {
			b.exitRecovery()
		}

		b.updateModelAndState(eventTime, uint64(priorInFlight))
		b.updateGains()
		b.updateControlParameters()
		b.setCwnd(uint64(priorInFlight))

		b.checkStartupDone()

		if b.state == bbrv3StateDrain && priorInFlight < protocol.ByteCount(b.inflight(1.0)) {
			b.enterProbeBw()
		}

		b.updateMaxBwFilter()

		if eventTime.Sub(b.minRttStamp) >= 10*time.Second && eventTime.Sub(b.probeRttDoneStamp) >= 10*time.Second {
			b.state = bbrv3StateProbeRTT
			b.pacingGain = 1.0
			b.cwndGain = 1.0
			b.minRtt = 10 * time.Second
			b.probeRttDoneStamp = eventTime
		}
	}
}

func (b *BBRv3Sender) calculateDeliveryRate(eventTime monotime.Time) uint64 {
	if len(b.ackinfo) == 0 {
		return 0
	}

	oldest := b.ackinfo[0]
	interval := time.Duration(eventTime - oldest.recordTime)
	if interval <= 0 {
		return 0
	}

	var totalBytes protocol.ByteCount
	for _, info := range b.ackinfo {
		totalBytes += info.ackedBytes
	}

	return uint64(float64(totalBytes) / interval.Seconds())
}

func (b *BBRv3Sender) updateMaxBwFilter() {
	if b.maxBwFilter.bw[1] == 0 {
		return
	}
	b.maxBwFilter.bw[0] = b.maxBwFilter.bw[1]
	b.maxBwFilter.bw[1] = 0
	b.maxBw = b.maxBwFilter.maxBw()
}

// OnCongestionEvent is called when there's a congestion event (packet loss)
func (b *BBRv3Sender) OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	b.totalBytesLost += uint64(lostBytes)
	if !b.inRecoveryMode && b.state != bbrv3StateStartup && b.state != bbrv3StateProbeRTT {
		b.inRecoveryMode = true
		b.pacingGain = 1.0
		b.enterRecovery(monotime.Now())
	}

	if b.cwnd > uint64(lostBytes) {
		b.cwnd -= uint64(lostBytes)
	} else {
		b.cwnd = b.config.minCwnd
	}
}

// OnRetransmissionTimeout is called when a retransmission timeout expires
func (b *BBRv3Sender) OnRetransmissionTimeout(packetsRetransmitted bool) {
	if packetsRetransmitted {
		oldConfig := b.config
		oldMaxDatagramSize := b.maxDatagramSize
		*b = *NewBBRv3Sender(oldMaxDatagramSize)
		b.config = oldConfig
		b.maxDatagramSize = oldMaxDatagramSize
	}
}

// SetMaxDatagramSize sets the maximum datagram size
func (b *BBRv3Sender) SetMaxDatagramSize(maxDatagramSize protocol.ByteCount) {
	b.maxDatagramSize = maxDatagramSize
	b.config.maxDatagramSize = uint64(maxDatagramSize)
	b.config.minCwnd = minPipeCwndInSmss * uint64(maxDatagramSize)
}

// GetCongestionWindow returns the current congestion window
func (b *BBRv3Sender) GetCongestionWindow() protocol.ByteCount {
	return protocol.ByteCount(b.cwnd)
}

// InRecovery returns whether we're in recovery mode
func (b *BBRv3Sender) InRecovery() bool {
	return b.inRecoveryMode
}

// InSlowStart returns whether we're in slow start
func (b *BBRv3Sender) InSlowStart() bool {
	return b.state == bbrv3StateStartup
}

func (b *BBRv3Sender) stateString() string {
	switch b.state {
	case bbrv3StateStartup:
		return "Startup"
	case bbrv3StateDrain:
		return "Drain"
	case bbrv3StateProbeBwDown:
		return "ProbeBW_Down"
	case bbrv3StateProbeBwCruise:
		return "ProbeBW_Cruise"
	case bbrv3StateProbeBwRefill:
		return "ProbeBW_Refill"
	case bbrv3StateProbeBwUp:
		return "ProbeBW_Up"
	case bbrv3StateProbeRTT:
		return "ProbeRTT"
	}
	return "Unknown"
}

func (b *BBRv3Sender) GetStats(bytesInFlight protocol.ByteCount) BBRv3Stats {
	b.smoothedRTT = b.lastNewMinRTT
	return BBRv3Stats{
		CongestionWindow: b.cwnd,
		PacingRate:       b.pacingRate,
		BytesInFlight:    uint64(bytesInFlight),
		TotalBytesSent:   b.totalBytesSent,
		TotalBytesLost:   b.totalBytesLost,
		MinRTT:           b.minRtt,
		MaxRTT:           b.probeRttMinDelay,
		LastRTT:          b.lastRTT,
		SmoothedRTT:      b.smoothedRTT,
		PacingGain:       b.pacingGain,
		CwndGain:         b.cwndGain,
		State:            b.stateString(),
		InRecovery:       b.inRecoveryMode,
		InSlowStart:      b.state == bbrv3StateStartup,
		MaxBandwidth:     b.maxBw,
	}
}
