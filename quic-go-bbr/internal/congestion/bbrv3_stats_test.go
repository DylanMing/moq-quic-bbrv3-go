package congestion

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func BenchmarkUpdateCWND(b *testing.B) {
	b.Run("V1-Mutex", func(b *testing.B) {
		stats := NewBBRv3Stats(BBRv3StatsConfig{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.UpdateCWND(uint64(i))
		}
	})

	b.Run("V2-Atomic-Channel", func(b *testing.B) {
		stats := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.UpdateCWND(uint64(i))
		}
	})

	b.Run("V3-Atomic-Pure", func(b *testing.B) {
		stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.UpdateCWND(uint64(i))
		}
	})
}

func BenchmarkUpdateRTT(b *testing.B) {
	rtt := 10 * time.Millisecond

	b.Run("V1-Mutex", func(b *testing.B) {
		stats := NewBBRv3Stats(BBRv3StatsConfig{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.UpdateRTT(rtt)
		}
	})

	b.Run("V2-Atomic-Channel", func(b *testing.B) {
		stats := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.UpdateRTT(rtt)
		}
	})

	b.Run("V3-Atomic-Pure", func(b *testing.B) {
		stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.UpdateRTT(rtt)
		}
	})
}

func BenchmarkAddBytesSent(b *testing.B) {
	b.Run("V1-Mutex", func(b *testing.B) {
		stats := NewBBRv3Stats(BBRv3StatsConfig{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.AddBytesSent(1400)
		}
	})

	b.Run("V2-Atomic-Channel", func(b *testing.B) {
		stats := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.AddBytesSent(1400)
		}
	})

	b.Run("V3-Atomic-Pure", func(b *testing.B) {
		stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{Enabled: false})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.AddBytesSent(1400)
		}
	})
}

func BenchmarkLog(b *testing.B) {
	discard := func([]byte) {}
	discardSnapshot := func(BBRv3StatsSnapshot) {}

	b.Run("V1-Mutex", func(b *testing.B) {
		stats := NewBBRv3Stats(BBRv3StatsConfig{
			Enabled:     true,
			LogInterval: time.Nanosecond,
			ConnID:      "test",
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.Log()
		}
	})

	b.Run("V2-Atomic-Channel", func(b *testing.B) {
		stats := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{
			Enabled:     true,
			LogInterval: time.Nanosecond,
			ConnID:      "test",
			LogFunc:     discardSnapshot,
		})
		defer stats.Stop()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.Log()
		}
	})

	b.Run("V3-Atomic-Pure", func(b *testing.B) {
		stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{
			Enabled:     true,
			LogInterval: time.Nanosecond,
			ConnID:      "test",
			LogCallback: discard,
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats.Log()
		}
	})
}

func BenchmarkConcurrent(b *testing.B) {
	b.Run("V1-Mutex", func(b *testing.B) {
		stats := NewBBRv3Stats(BBRv3StatsConfig{Enabled: false})
		b.RunParallel(func(pb *testing.PB) {
			for i := 0; pb.Next(); i++ {
				stats.UpdateCWND(uint64(i))
				stats.UpdateRTT(time.Millisecond)
				stats.AddBytesSent(1400)
			}
		})
	})

	b.Run("V2-Atomic-Channel", func(b *testing.B) {
		stats := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{Enabled: false})
		defer stats.Stop()
		b.RunParallel(func(pb *testing.PB) {
			for i := 0; pb.Next(); i++ {
				stats.UpdateCWND(uint64(i))
				stats.UpdateRTT(time.Millisecond)
				stats.AddBytesSent(1400)
			}
		})
	})

	b.Run("V3-Atomic-Pure", func(b *testing.B) {
		stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{Enabled: false})
		b.RunParallel(func(pb *testing.PB) {
			for i := 0; pb.Next(); i++ {
				stats.UpdateCWND(uint64(i))
				stats.UpdateRTT(time.Millisecond)
				stats.AddBytesSent(1400)
			}
		})
	})
}

func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("V1-Log", func(b *testing.B) {
		stats := NewBBRv3Stats(BBRv3StatsConfig{Enabled: true, LogInterval: time.Nanosecond})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			stats.Log()
		}
	})

	b.Run("V2-Log", func(b *testing.B) {
		stats := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{
			Enabled:     true,
			LogInterval: time.Nanosecond,
			LogFunc:     func(BBRv3StatsSnapshot) {},
		})
		defer stats.Stop()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			stats.Log()
		}
	})

	b.Run("V3-Log", func(b *testing.B) {
		stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{
			Enabled:     true,
			LogInterval: time.Nanosecond,
			LogCallback: func([]byte) {},
		})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			stats.Log()
		}
	})
}

func TestStatsCorrectness(t *testing.T) {
	t.Run("V1", func(t *testing.T) {
		stats := NewBBRv3Stats(BBRv3StatsConfig{Enabled: true, ConnID: "test"})

		stats.UpdateCWND(1000)
		stats.AddBytesSent(1400)
		stats.AddBytesSent(1400)
		stats.AddBytesLost(100)
		stats.UpdateRTT(10 * time.Millisecond)
		stats.UpdateRTT(20 * time.Millisecond)
		stats.IncrementRetransmit()
		stats.UpdateState("Startup")

		snapshot := stats.GetStatsSnapshot()

		if snapshot.CWND != 1000 {
			t.Errorf("CWND mismatch: got %d, want 1000", snapshot.CWND)
		}
		if snapshot.BytesSent != 2800 {
			t.Errorf("BytesSent mismatch: got %d, want 2800", snapshot.BytesSent)
		}
		if snapshot.BytesLost != 100 {
			t.Errorf("BytesLost mismatch: got %d, want 100", snapshot.BytesLost)
		}
		if snapshot.RetransmitCount != 1 {
			t.Errorf("RetransmitCount mismatch: got %d, want 1", snapshot.RetransmitCount)
		}
		if snapshot.State != "Startup" {
			t.Errorf("State mismatch: got %s, want Startup", snapshot.State)
		}
		if snapshot.MinRTT != 10*time.Millisecond {
			t.Errorf("MinRTT mismatch: got %v, want 10ms", snapshot.MinRTT)
		}
	})

	t.Run("V2", func(t *testing.T) {
		stats := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{Enabled: true, ConnID: "test"})
		defer stats.Stop()

		stats.UpdateCWND(1000)
		stats.AddBytesSent(1400)
		stats.AddBytesSent(1400)
		stats.AddBytesLost(100)
		stats.UpdateRTT(10 * time.Millisecond)
		stats.UpdateRTT(20 * time.Millisecond)
		stats.IncrementRetransmit()
		stats.UpdateState("Startup")

		snapshot := stats.GetStatsSnapshot()

		if snapshot.CWND != 1000 {
			t.Errorf("CWND mismatch: got %d, want 1000", snapshot.CWND)
		}
		if snapshot.BytesSent != 2800 {
			t.Errorf("BytesSent mismatch: got %d, want 2800", snapshot.BytesSent)
		}
		if snapshot.BytesLost != 100 {
			t.Errorf("BytesLost mismatch: got %d, want 100", snapshot.BytesLost)
		}
		if snapshot.RetransmitCount != 1 {
			t.Errorf("RetransmitCount mismatch: got %d, want 1", snapshot.RetransmitCount)
		}
		if snapshot.State != "Startup" {
			t.Errorf("State mismatch: got %s, want Startup", snapshot.State)
		}
	})

	t.Run("V3", func(t *testing.T) {
		stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{Enabled: true, ConnID: "test"})

		stats.UpdateCWND(1000)
		stats.AddBytesSent(1400)
		stats.AddBytesSent(1400)
		stats.AddBytesLost(100)
		stats.UpdateRTT(10 * time.Millisecond)
		stats.UpdateRTT(20 * time.Millisecond)
		stats.IncrementRetransmit()
		stats.UpdateStateByName("Startup")

		snapshot := stats.GetStatsSnapshot()

		if snapshot.CWND != 1000 {
			t.Errorf("CWND mismatch: got %d, want 1000", snapshot.CWND)
		}
		if snapshot.BytesSent != 2800 {
			t.Errorf("BytesSent mismatch: got %d, want 2800", snapshot.BytesSent)
		}
		if snapshot.BytesLost != 100 {
			t.Errorf("BytesLost mismatch: got %d, want 100", snapshot.BytesLost)
		}
		if snapshot.RetransmitCount != 1 {
			t.Errorf("RetransmitCount mismatch: got %d, want 1", snapshot.RetransmitCount)
		}
		if snapshot.State != "Startup" {
			t.Errorf("State mismatch: got %s, want Startup", snapshot.State)
		}
	})
}

func TestConcurrentSafety(t *testing.T) {
	for _, version := range []string{"V1", "V2", "V3"} {
		t.Run(version, func(t *testing.T) {
			var stats interface {
				UpdateCWND(uint64)
				AddBytesSent(uint64)
				UpdateRTT(time.Duration)
			}

			switch version {
			case "V1":
				stats = NewBBRv3Stats(BBRv3StatsConfig{Enabled: false})
			case "V2":
				s := NewBBRv3StatsOptimized(BBRv3StatsConfigOptimized{Enabled: false})
				defer s.Stop()
				stats = s
			case "V3":
				stats = NewBBRv3StatsV2(BBRv3StatsConfigV2{Enabled: false})
			}

			var wg sync.WaitGroup
			for i := 0; i < 100; i++ {
				wg.Add(3)
				go func() {
					defer wg.Done()
					for j := 0; j < 1000; j++ {
						stats.UpdateCWND(uint64(j))
					}
				}()
				go func() {
					defer wg.Done()
					for j := 0; j < 1000; j++ {
						stats.AddBytesSent(1400)
					}
				}()
				go func() {
					defer wg.Done()
					for j := 0; j < 1000; j++ {
						stats.UpdateRTT(time.Duration(j) * time.Microsecond)
					}
				}()
			}
			wg.Wait()
		})
	}
}

func TestJSONOutput(t *testing.T) {
	var output []byte
	stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{
		Enabled: true,
		ConnID:  "test-conn",
		LogCallback: func(data []byte) {
			output = data
		},
	})

	stats.UpdateCWND(12800)
	stats.AddBytesSent(10000)
	stats.UpdateRTT(15 * time.Millisecond)
	stats.UpdateStateByName("ProbeBW-Up")
	stats.Log()

	if len(output) == 0 {
		t.Fatal("No JSON output")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if result["cwnd"].(float64) != 12800 {
		t.Errorf("CWND in JSON mismatch: %v", result["cwnd"])
	}
	if result["state"].(string) != "ProbeBW-Up" {
		t.Errorf("State in JSON mismatch: %v", result["state"])
	}

	fmt.Printf("JSON Output: %s\n", string(output))
}

func ExampleBBRv3StatsV2() {
	stats := NewBBRv3StatsV2(BBRv3StatsConfigV2{
		Enabled:     true,
		LogInterval: time.Second,
		ConnID:      "example-conn",
		LogCallback: func(jsonData []byte) {
			fmt.Printf("Stats: %s\n", jsonData)
			os.Stdout.Write(jsonData)
			os.Stdout.Write([]byte("\n"))
		},
	})

	stats.UpdateCWND(12800)
	stats.AddBytesSent(10000)
	stats.UpdateRTT(15 * time.Millisecond)
	stats.UpdateStateByName("Startup")

	if stats.ShouldLog() {
		stats.Log()
	}
}
