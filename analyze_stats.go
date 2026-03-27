package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type StatsRecord struct {
	Timestamp      time.Time `json:"ts"`
	ConnID         string    `json:"conn_id"`
	Algorithm      string    `json:"algo"`
	State          string    `json:"state"`
	CWND           uint64    `json:"cwnd"`
	SSTHRESH       uint64    `json:"ssthresh"`
	BytesSent      uint64    `json:"bytes_sent"`
	BytesLost      uint64    `json:"bytes_lost"`
	RetransmitCount uint64   `json:"retrans"`
	MinRTT         float64   `json:"min_rtt_us"`
	AvgRTT         float64   `json:"avg_rtt_us"`
	CurrentRTT     float64   `json:"cur_rtt_us"`
	TXRate         uint64    `json:"tx_rate"`
	Bandwidth      uint64    `json:"bw"`
	MaxBandwidth   uint64    `json:"max_bw"`
	PacingRate     uint64    `json:"pacing_rate"`
	Inflight       uint64    `json:"inflight"`
}

type TestResult struct {
	Algorithm       string
	TestDuration    time.Duration
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
	if len(os.Args) < 2 {
		fmt.Println("用法: go run analyze_stats.go <stats_log_directory>")
		fmt.Println("示例: go run analyze_stats.go test_results")
		os.Exit(1)
	}

	logDir := os.Args[1]
	results := make(map[string]*TestResult)

	algorithms := []string{"CUBIC", "BBRv1", "BBRv3"}

	for _, algo := range algorithms {
		logFile := filepath.Join(logDir, fmt.Sprintf("%s_stats.log", algo))
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			fmt.Printf("警告: 未找到 %s 的日志文件: %s\n", algo, logFile)
			continue
		}

		result := analyzeLogFile(logFile, algo)
		results[algo] = result
	}

	if len(results) == 0 {
		fmt.Println("错误: 未找到任何日志文件")
		os.Exit(1)
	}

	generateReport(results, logDir)
}

func analyzeLogFile(logFile, algorithm string) *TestResult {
	result := &TestResult{
		Algorithm: algorithm,
		Records:   make([]StatsRecord, 0),
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Printf("读取文件失败: %v\n", err)
		return result
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, `"algo"`) || strings.Contains(line, `"cwnd"`) {
			var record StatsRecord
			if err := json.Unmarshal([]byte(line), &record); err == nil {
				record.Algorithm = algorithm
				result.Records = append(result.Records, record)
			}
		}
	}

	if len(result.Records) == 0 {
		return result
	}

	sort.Slice(result.Records, func(i, j int) bool {
		return result.Records[i].Timestamp.Before(result.Records[j].Timestamp)
	})

	if len(result.Records) >= 2 {
		result.TestDuration = result.Records[len(result.Records)-1].Timestamp.Sub(result.Records[0].Timestamp)
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
		if result.MaxRTT == 0 || r.CurrentRTT > result.MaxRTT {
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

	if result.TestDuration.Seconds() > 0 {
		result.AvgThroughput = float64(result.TotalBytesSent) / result.TestDuration.Seconds()
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
	reportFile := filepath.Join(outputDir, "CONGESTION_COMPARISON_REPORT.md")
	f, err := os.Create(reportFile)
	if err != nil {
		fmt.Printf("创建报告失败: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# 拥塞控制算法对比测试报告\n\n")
	fmt.Fprintf(f, "## 测试环境\n\n")
	fmt.Fprintf(f, "- **测试时间**: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "- **网络环境**: 本地回环 (localhost)\n")
	fmt.Fprintf(f, "- **测试工具**: moq-go + quic-go-bbr\n")
	fmt.Fprintf(f, "- **测试协议**: QUIC (MOQ)\n\n")

	fmt.Fprintf(f, "## 测试结果汇总\n\n")
	fmt.Fprintf(f, "| 指标 |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if results[algo] != nil {
			fmt.Fprintf(f, " %s |", algo)
		}
	}
	fmt.Fprintf(f, "\n|------|")
	for range results {
		fmt.Fprintf(f, "-------|")
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 测试时长 |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.1fs |", r.TestDuration.Seconds())
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 总发送字节 (MB) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.2f |", float64(r.TotalBytesSent)/1024/1024)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 平均吞吐量 (Mbps) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.2f |", r.AvgThroughput*8/1000000)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 峰值吞吐量 (Mbps) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.2f |", r.MaxThroughput*8/1000000)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 丢包字节数 |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %d |", r.TotalBytesLost)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 丢包率 (%%) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.4f |", r.PacketLossRate)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 重传次数 |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %d |", r.TotalRetrans)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 最小 RTT (ms) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.3f |", r.MinRTT/1000)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 平均 RTT (ms) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.3f |", r.AvgRTT/1000)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 最大 RTT (ms) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.3f |", r.MaxRTT/1000)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 平均 CWND (bytes) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %.0f |", r.AvgCWND)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "| 最大 CWND (bytes) |")
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, " %d |", r.MaxCWND)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "\n## 详细分析\n\n")

	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok {
			fmt.Fprintf(f, "### %s\n\n", algo)
			fmt.Fprintf(f, "**基本统计**:\n")
			fmt.Fprintf(f, "- 测试时长: %v\n", r.TestDuration)
			fmt.Fprintf(f, "- 数据记录数: %d\n", len(r.Records))
			fmt.Fprintf(f, "\n**吞吐量**:\n")
			fmt.Fprintf(f, "- 总发送字节: %d bytes (%.2f MB)\n", r.TotalBytesSent, float64(r.TotalBytesSent)/1024/1024)
			fmt.Fprintf(f, "- 平均吞吐量: %.2f bytes/s (%.2f Mbps)\n", r.AvgThroughput, r.AvgThroughput*8/1000000)
			fmt.Fprintf(f, "- 峰值吞吐量: %.2f bytes/s (%.2f Mbps)\n", r.MaxThroughput, r.MaxThroughput*8/1000000)
			fmt.Fprintf(f, "\n**丢包与重传**:\n")
			fmt.Fprintf(f, "- 丢包字节数: %d bytes\n", r.TotalBytesLost)
			fmt.Fprintf(f, "- 丢包率: %.4f%%\n", r.PacketLossRate)
			fmt.Fprintf(f, "- 重传次数: %d\n", r.TotalRetrans)
			fmt.Fprintf(f, "\n**延迟**:\n")
			fmt.Fprintf(f, "- 最小 RTT: %.3f ms\n", r.MinRTT/1000)
			fmt.Fprintf(f, "- 平均 RTT: %.3f ms\n", r.AvgRTT/1000)
			fmt.Fprintf(f, "- 最大 RTT: %.3f ms\n", r.MaxRTT/1000)
			fmt.Fprintf(f, "\n**拥塞窗口**:\n")
			fmt.Fprintf(f, "- 平均 CWND: %.0f bytes\n", r.AvgCWND)
			fmt.Fprintf(f, "- 最大 CWND: %d bytes\n\n", r.MaxCWND)
		}
	}

	fmt.Fprintf(f, "## 性能对比分析\n\n")

	bestThroughput := ""
	bestRTT := ""
	bestLoss := ""
	
	for _, algo := range []string{"CUBIC", "BBRv1", "BBRv3"} {
		if r, ok := results[algo]; ok && len(r.Records) > 0 {
			if bestThroughput == "" || r.AvgThroughput > results[bestThroughput].AvgThroughput {
				bestThroughput = algo
			}
			if bestRTT == "" || (r.AvgRTT < results[bestRTT].AvgRTT && r.AvgRTT > 0) {
				bestRTT = algo
			}
			if bestLoss == "" || r.PacketLossRate < results[bestLoss].PacketLossRate {
				bestLoss = algo
			}
		}
	}

	fmt.Fprintf(f, "### 最优算法\n\n")
	if bestThroughput != "" {
		fmt.Fprintf(f, "| 指标 | 最优算法 | 数值 |\n")
		fmt.Fprintf(f, "|------|----------|------|\n")
		fmt.Fprintf(f, "| 吞吐量 | %s | %.2f Mbps |\n", bestThroughput, results[bestThroughput].AvgThroughput*8/1000000)
		fmt.Fprintf(f, "| 延迟 | %s | %.3f ms |\n", bestRTT, results[bestRTT].AvgRTT/1000)
		fmt.Fprintf(f, "| 丢包率 | %s | %.4f%% |\n", bestLoss, results[bestLoss].PacketLossRate)
	}

	fmt.Fprintf(f, "\n### 算法特点分析\n\n")
	fmt.Fprintf(f, "#### CUBIC\n")
	fmt.Fprintf(f, "- 基于丢包的拥塞控制算法\n")
	fmt.Fprintf(f, "- 在低延迟、低丢包环境下表现稳定\n")
	fmt.Fprintf(f, "- 拥塞窗口增长采用三次函数\n\n")

	fmt.Fprintf(f, "#### BBRv1\n")
	fmt.Fprintf(f, "- 基于带宽和 RTT 的拥塞控制\n")
	fmt.Fprintf(f, "- 更好地探测可用带宽\n")
	fmt.Fprintf(f, "- 在高延迟网络中表现更好\n\n")

	fmt.Fprintf(f, "#### BBRv3\n")
	fmt.Fprintf(f, "- BBR 的最新版本\n")
	fmt.Fprintf(f, "- 改进了带宽探测和 RTT 估计\n")
	fmt.Fprintf(f, "- 更精确的带宽估计，更好的公平性\n\n")

	fmt.Fprintf(f, "## 结论与建议\n\n")
	fmt.Fprintf(f, "### 场景建议\n\n")
	fmt.Fprintf(f, "1. **低延迟、低丢包环境**: 推荐使用 CUBIC\n")
	fmt.Fprintf(f, "2. **高延迟、高丢包环境**: 推荐使用 BBRv3\n")
	fmt.Fprintf(f, "3. **混合环境**: 推荐使用 BBRv1 作为平衡选择\n\n")

	fmt.Fprintf(f, "### 改进建议\n\n")
	fmt.Fprintf(f, "1. 在实际网络环境中进行更长时间的测试\n")
	fmt.Fprintf(f, "2. 模拟不同的网络条件（带宽限制、延迟、丢包）\n")
	fmt.Fprintf(f, "3. 进行多流并发测试以评估公平性\n")
	fmt.Fprintf(f, "4. 与其他 QUIC 实现进行对比测试\n")

	fmt.Printf("\n测试报告已保存到: %s\n", reportFile)
}
