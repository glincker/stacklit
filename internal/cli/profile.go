package cli

import (
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"
)

// startProfile writes cpu and heap profiles into STACKLIT_PPROF when set, so
// scale regressions can be diagnosed on real repos without a debug build.
func startProfile() func() {
	dir := os.Getenv("STACKLIT_PPROF")
	if dir == "" {
		return func() {}
	}
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.Create(filepath.Join(dir, "cpu.pprof"))
	if err != nil {
		return func() {}
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		return func() {}
	}
	go trackPeakHeap(dir)
	var once sync.Once
	stop := func() { once.Do(flush(f, dir)) }
	// Flush on interrupt so a run killed by a timeout still leaves a profile.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		stop()
		os.Exit(130)
	}()
	return stop
}

func flush(f *os.File, dir string) func() {
	return func() {
		pprof.StopCPUProfile()
		_ = f.Close()
		if h, err := os.Create(filepath.Join(dir, "heap.pprof")); err == nil {
			_ = pprof.Lookup("allocs").WriteTo(h, 0)
			_ = h.Close()
		}
	}
}

// trackPeakHeap keeps the in-use heap profile from the moment of highest
// heap, because the end-of-run profile misses the peak that sets RSS.
func trackPeakHeap(dir string) {
	var peak uint64
	var ms runtime.MemStats
	for {
		time.Sleep(200 * time.Millisecond)
		runtime.ReadMemStats(&ms)
		if ms.HeapInuse <= peak {
			continue
		}
		peak = ms.HeapInuse
		if h, err := os.Create(filepath.Join(dir, "heap-peak.pprof")); err == nil {
			_ = pprof.Lookup("heap").WriteTo(h, 0)
			_ = h.Close()
		}
	}
}
