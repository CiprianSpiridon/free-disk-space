package size

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func benchmarkStrategies(b *testing.B, root string) {
	b.Helper()
	for _, mode := range []string{"walkdir-stat", "cache-stat", "cache-bulk", "cache-bulk-4", "cache-bulk-10", "parallel-bulk-4"} {
		b.Run(mode, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if mode == "walkdir-stat" {
					Of(root)
					continue
				}
				if mode == "parallel-bulk-4" {
					entries, err := os.ReadDir(root)
					if err != nil {
						b.Fatal(err)
					}
					jobs := make(chan string)
					var wg sync.WaitGroup
					for w := 0; w < 4; w++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							for path := range jobs {
								s := NewScanner()
								s.Workers = 1
								r := s.Of(path)
								if r.Err != nil {
									b.Error(r.Err)
								}
							}
						}()
					}
					for _, e := range entries {
						jobs <- filepath.Join(root, e.Name())
					}
					close(jobs)
					wg.Wait()
					continue
				}
				s := NewScanner()
				if mode == "cache-bulk-4" {
					s.Workers = 4
				}
				if mode == "cache-bulk-10" {
					s.Workers = 10
				}
				if mode == "cache-stat" {
					s.readEntries = readStatDir
				}
				r := s.Of(root)
				if r.Err != nil {
					b.Fatal(r.Err)
				}
				if strings.HasPrefix(mode, "cache-bulk") && s.BulkFallbacks != 0 {
					b.Fatalf("native scan unexpectedly fell back %d times", s.BulkFallbacks)
				}
			}
		})
	}
}

func BenchmarkTraversalStrategies(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 80; i++ {
		d := filepath.Join(root, fmt.Sprint(i))
		os.Mkdir(d, 0755)
		for j := 0; j < 300; j++ {
			if err := os.WriteFile(filepath.Join(d, fmt.Sprint(j)), []byte("content"), 0644); err != nil {
				b.Fatal(err)
			}
		}
	}
	benchmarkStrategies(b, root)
}

func BenchmarkRealTree(b *testing.B) {
	root := os.Getenv("FREEDISK_BENCH_PATH")
	if root == "" {
		b.Skip("set FREEDISK_BENCH_PATH to opt into a real-tree read-only benchmark")
	}
	benchmarkStrategies(b, root)
}

// Two passes in opposite orders reduce the advantage of running last against
// warm kernel metadata. Each sub-benchmark creates a fresh per-scan cache.
func BenchmarkWorkerCounts(b *testing.B) {
	root := os.Getenv("FREEDISK_BENCH_PATH")
	if root == "" {
		b.Skip("set FREEDISK_BENCH_PATH to opt into a real-tree read-only benchmark")
	}
	var expected *Result
	var expectedVisits int
	for round, order := range [][]int{{1, 2, 4, 5, 10}, {10, 5, 4, 2, 1}} {
		for _, workers := range order {
			b.Run(fmt.Sprintf("round-%d/workers-%d", round+1, workers), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					s := NewScanner()
					s.Workers = workers
					r := s.Of(root)
					if r.Err != nil || len(r.Unreadable) != 0 || s.BulkFallbacks != 0 {
						b.Fatalf("incomplete traversal: error=%v unreadable=%d fallbacks=%d", r.Err, len(r.Unreadable), s.BulkFallbacks)
					}
					if expected == nil {
						copy := r
						expected = &copy
						expectedVisits = s.Visits
					} else if r.Allocated != expected.Allocated || r.Apparent != expected.Apparent || r.Private != expected.Private || r.PrivateKnown != expected.PrivateKnown || !r.Newest.Equal(expected.Newest) || s.Visits != expectedVisits {
						b.Fatalf("coverage changed: bytes=%d/%d apparent=%d/%d private=%d/%d newest=%s/%s visits=%d/%d", r.Allocated, expected.Allocated, r.Apparent, expected.Apparent, r.Private, expected.Private, r.Newest, expected.Newest, s.Visits, expectedVisits)
					}
					b.ReportMetric(float64(s.Visits), "entries/op")
				}
			})
		}
	}
}
