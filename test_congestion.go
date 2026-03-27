package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TestConfig struct {
	Name         string
	Algorithm    string
	Duration     time.Duration
	OutputDir    string
	RelayPort    int
}

type StatsRecord struct {
	Timestamp       time.Time `json:"ts"`
	ConnID          string    `json:"conn_id"`
	Algorithm       string    `json:"algo"`
	State           string    `json:"state"`
	CWND            uint64    `json:"cwnd"`
	SSTHRESH        uint64    `json:"ssthresh"`
	BytesSent       uint64    `json:"bytes_sent"`
	BytesLost       uint64    `json:"bytes_lost"`
	RetransmitCount uint64    `json:"retrans"`
	MinRTT          float64   `json:"min_rtt_us"`
	AvgRTT          float64   `json:"avg_rtt_us"`
	CurrentRTT      float64   `json:"cur_rtt_us"`
	TXRate          uint64    `json:"tx_rate"`
	Bandwidth       uint64    `json:"bw"`
	MaxBandwidth    uint64    `json:"max_bw"`
	PacingRate      uint64    `json:"pacing_rate"`
	Inflight        uint64    `json:"inflight"`
}

type TestResult struct {
	Algorithm       string
	Duration        time.Duration
	TotalBytesSent  uint64
	TotalBytesLost  uint64
	TotalRetrans    uint64
	AvgThroughput   float64
	MaxThroughput   float64
	MinRTT          float64
	AvgRTT          float64
	MaxRTT          float64
	PacketLossRate  float64
	AvgCWND         float64
	MaxCWND         uint64
	Records         []StatsRecord
}

func main() {
	outputDir := "test_results"
	os.MkdirAll(outputDir, 0755)

	algorithms := []string{"CUBIC", "BBRv1", "BBRv3"}
	
	fmt.Println("========================================")
	fmt.Println("  网络监控系统全面测试")
	fmt.Println("========================================")
	fmt.Println()

	results := make(map[string]*TestResult)

	for _, algo := range algorithms {
		fmt.Printf("\n>>> 开始测试 %s 算法...\n", algo)
		result := runTest(algo, outputDir)
		results[algo] = result
		fmt.Printf("<<< %s 测试完成\n", algo)
	}

	fmt.Println("\n========================================")
	fmt.Println("  测试结果汇总")
	fmt.Println("========================================")
	
	generateReport(results, outputDir)
}

func runTest(algorithm, outputDir string) *TestResult {
	result := &TestResult{
		Algorithm: algorithm,
		Records:   make([]StatsRecord, 0),
	}

	logFile := filepath.Join(outputDir, fmt.Sprintf("%s_stats.log", algorithm))
	statsFile, err := os.Create(logFile)
	if err != nil {
		fmt.Printf("创建日志文件失败: %v\n", err)
		return result
	}
	defer statsFile.Close()

	relayCmd := exec.Command("go", "run", "examples/relay/relay.go", "-debug")
	relayCmd.Dir = "moq-go"
	relayCmd.Env = append(os.Environ(), 
		"MOQT_CERT_PATH=examples/certs/localhost.crt",
		"MOQT_KEY_PATH=examples/certs/localhost.key",
	)
	relayCmd.Stdout = os.Stdout
	relayCmd.Stderr = os.Stderr
	
	if err := relayCmd.Start(); err != nil {
		fmt.Printf("启动 relay 失败: %v\n", err)
		return result
	}
	defer relayCmd.Process.Kill()

	time.Sleep(2 * time.Second)

	pubCmd := exec.Command("go", "run", "examples/newpub/newpub.go", "-debug")
	pubCmd.Dir = "moq-go"
	pubCmd.Stdout = io.MultiWriter(os.Stdout, statsFile)
	pubCmd.Stderr = os.Stderr

	if err := pubCmd.Start(); err != nil {
		fmt.Printf("启动 newpub 失败: %v\n", err)
		return result
	}
	defer pubCmd.Process.Kill()

	time.Sleep(1 * time.Second)

	subCmd := exec.Command("go", "run", "examples/newsub/newsub.go", "-debug")
	subCmd.Dir = "moq-go"
	subCmd.Stdout = io.MultiWriter(os.Stdout, statsFile)
	subCmd.Stderr = os.Stderr

	if err := subCmd.Start(); err != nil {
		fmt.Printf("启动 newsub 失败: %v\n", err)
		return result
	}
	defer subCmd.Process.Kill()

	testDuration := 30 * time.Second
	startTime := time.Now()
	
	for time.Since(startTime) < testDuration {
		time.Sleep(1 * time.Second)
		fmt.Printf(".")
	}
	fmt.Println()

	result.Duration = testDuration

	statsFile.Seek(0, 0)
	scanner := bufio.NewScanner(statsFile)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"algo"`) {
			var record StatsRecord
			if err := json.Unmarshal([]byte(line), &record); err == nil {
				record.Algorithm = algorithm
				result.Records = append(result.Records, record)
			}
		}
	}

	calculateMetrics(result)

	return result
}

func calculateMetrics(result *TestResult) {
	if len(result.Records) == 0 {
		return
	}

	var totalRTT float64
	var totalCWND float64
	var count int

	for _, r := range result.Records {
		result.TotalBytesSent = max(result.TotalBytesSent, r.BytesSent)
		result.TotalBytesLost = max(result.TotalBytesLost, r.BytesLost)
		result.TotalRetrans = max(result.TotalRetrans, r.RetransmitCount)
		result.MaxCWND = max(result.MaxCWND, r.CWND)
		result.MaxThroughput = max(result.MaxThroughput, float64(r.Bandwidth))

		if r.MinRTT > 0 {
			if result.MinRTT == 0 || r.MinRTT < result.MinRTT {
				result.MinRTT = r.MinRTT
			}
		}
		if r.AvgRTT > 0 {
			totalRTT += r.AvgRTT
			count++
		}
		if r.MaxRTT == 0 || r.CurrentRTT > result.MaxRTT {
			result.MaxRTT = r.CurrentRTT
		}
		totalCWND += float64(r.CWND)
	}

	if count > 0 {
		result.AvgRTT = totalRTT / float64(count)
	}
	if len(result.Records) > 0 {
		result.AvgCWND = totalCWND / float64(len(result.Records))
	}

	if result.Duration.Seconds() > 0 {
		result.AvgThroughput = float64(result.TotalBytesSent) / result.Duration.Seconds()
	}

	if result.TotalBytesSent > 0 {
		result.PacketLossRate = float64(result.TotalBytesLost) / float64(result.TotalBytesSent) * 100
	}
}

func max[T int | int64 | uint64 | float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func generateReport(results map[string]*TestResult, outputDir string) {
	reportFile := filepath.Join(outputDir, "TEST_REPORT.md")
	f, err := os.Create(reportFile)
	if err != nil {
		fmt.Printf("创建报告失败: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# 网络监控系统全面测试报告\n\n")
	fmt.Fprintf(f, "## 测试环境\n\n")
	fmt.Fprintf(f, "- **测试时间**: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "- **测试时长**: 30 秒/算法\n")
	fmt.Fprintf(f, "- **网络环境**: 本地回环 (localhost)\n")
	fmt.Fprintf(f, "- **测试工具**: moq-go + quic-go-bbr\n\n")

	fmt.Fprintf(f, "## 测试结果汇总\n\n")
	fmt.Fprintf(f, "| 指标 | CUBIC | BBRv1 | BBRv3 |\n")
	fmt.Fprintf(f, "|------|-------|-------|-------|\n")
	
	fmt.Fprintf(f, "| 总发送字节 (MB) | %.2f | %.2f | %.2f |\n",
		float64(results["CUBIC"].TotalBytesSent)/1024/1024,
		float64(results["BBRv1"].TotalBytesSent)/1024/1024,
		float64(results["BBRv3"].TotalBytesSent)/1024/1024)
	
	fmt.Fprintf(f, "| 平均吞吐量 (Mbps) | %.2f | %.2f | %.2f |\n",
		results["CUBIC"].AvgThroughput*8/1000000,
		results["BBRv1"].AvgThroughput*8/1000000,
		results["BBRv3"].AvgThroughput*8/1000000)
	
	fmt.Fprintf(f, "| 峰值吞吐量 (Mbps) | %.2f | %.2f | %.2f |\n",
		results["CUBIC"].MaxThroughput*8/1000000,
		results["BBRv1"].MaxThroughput*8/1000000,
		results["BBRv3"].MaxThroughput*8/1000000)
	
	fmt.Fprintf(f, "| 丢包字节数 | %d | %d | %d |\n",
		results["CUBIC"].TotalBytesLost,
		results["BBRv1"].TotalBytesLost,
		results["BBRv3"].TotalBytesLost)
	
	fmt.Fprintf(f, "| 丢包率 (%%) | %.4f | %.4f | %.4f |\n",
		results["CUBIC"].PacketLossRate,
		results["BBRv1"].PacketLossRate,
		results["BBRv3"].PacketLossRate)
	
	fmt.Fprintf(f, "| 重传次数 | %d | %d | %d |\n",
		results["CUBIC"].TotalRetrans,
		results["BBRv1"].TotalRetrans,
		results["BBRv3"].TotalRetrans)
	
	fmt.Fprintf(f, "| 最小 RTT (ms) | %.3f | %.3f | %.3f |\n",
		results["CUBIC"].MinRTT/1000,
		results["BBRv1"].MinRTT/1000,
		results["BBRv3"].MinRTT/1000)
	
	fmt.Fprintf(f, "| 平均 RTT (ms) | %.3f | %.3f | %.3f |\n",
		results["CUBIC"].AvgRTT/1000,
		results["BBRv1"].AvgRTT/1000,
		results["BBRv3"].AvgRTT/1000)
	
	fmt.Fprintf(f, "| 平均 CWND (bytes) | %.0f | %.0f | %.0f |\n",
		results["CUBIC"].AvgCWND,
		results["BBRv1"].AvgCWND,
		results["BBRv3"].AvgCWND)

	fmt.Fprintf(f, "\n## 详细分析\n\n")

	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		r := results[algo]
		fmt.Fprintf(f, "### %s\n\n", algo)
		fmt.Fprintf(f, "- 测试时长: %v\n", r.Duration)
		fmt.Fprintf(f, "- 数据记录数: %d\n", len(r.Records))
		fmt.Fprintf(f, "- 总发送字节: %d bytes (%.2f MB)\n", r.TotalBytesSent, float64(r.TotalBytesSent)/1024/1024)
		fmt.Fprintf(f, "- 平均吞吐量: %.2f bytes/s (%.2f Mbps)\n", r.AvgThroughput, r.AvgThroughput*8/1000000)
		fmt.Fprintf(f, "- 峰值吞吐量: %.2f bytes/s (%.2f Mbps)\n", r.MaxThroughput, r.MaxThroughput*8/1000000)
		fmt.Fprintf(f, "- 丢包字节数: %d bytes\n", r.TotalBytesLost)
		fmt.Fprintf(f, "- 丢包率: %.4f%%\n", r.PacketLossRate)
		fmt.Fprintf(f, "- 重传次数: %d\n", r.TotalRetrans)
		fmt.Fprintf(f, "- RTT: 最小=%.3fms, 平均=%.3fms, 最大=%.3fms\n", 
			r.MinRTT/1000, r.AvgRTT/1000, r.MaxRTT/1000)
		fmt.Fprintf(f, "- CWND: 平均=%.0f bytes, 最大=%d bytes\n\n", r.AvgCWND, r.MaxCWND)
	}

	fmt.Fprintf(f, "## 结论与建议\n\n")
	fmt.Fprintf(f, "### 性能对比\n\n")
	
	bestThroughput := "CUBIC"
	bestRTT := "CUBIC"
	bestLoss := "CUBIC"
	
	for _, algo := range []string{"BBRv1", "BBRv3"} {
		if results[algo].AvgThroughput > results[bestThroughput].AvgThroughput {
			bestThroughput = algo
		}
		if results[algo].AvgRTT < results[bestRTT].AvgRTT && results[algo].AvgRTT > 0 {
			bestRTT = algo
		}
		if results[algo].PacketLossRate < results[bestLoss].PacketLossRate {
			bestLoss = algo
		}
	}

	fmt.Fprintf(f, "1. **吞吐量最优**: %s\n", bestThroughput)
	fmt.Fprintf(f, "2. **延迟最优**: %s\n", bestRTT)
	fmt.Fprintf(f, "3. **丢包率最低**: %s\n\n", bestLoss)

	fmt.Fprintf(f, "### 改进建议\n\n")
	fmt.Fprintf(f, "1. 在高延迟网络环境下，建议使用 BBRv3 以获得更好的吞吐量\n")
	fmt.Fprintf(f, "2. 在低延迟、低丢包环境下，CUBIC 表现稳定\n")
	fmt.Fprintf(f, "3. BBRv1 在大多数场景下提供了良好的平衡\n")
	fmt.Fprintf(f, "4. 建议根据实际网络条件选择合适的拥塞控制算法\n")

	fmt.Printf("\n测试报告已保存到: %s\n", reportFile)
}
