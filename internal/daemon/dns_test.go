package daemon

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDNSFlusher(t *testing.T) {
	tests := []FlushMethod{
		FlushMethodAuto,
		FlushMethodDscacheutil,
		FlushMethodKillall,
		FlushMethodBoth,
		FlushMethodSystemd,
		FlushMethodNscd,
	}

	for _, method := range tests {
		t.Run(string(method), func(t *testing.T) {
			flusher := NewDNSFlusher(method)
			assert.NotNil(t, flusher)
			assert.Equal(t, method, flusher.method)
		})
	}
}

func TestDNSFlusher_DetectMethod(t *testing.T) {
	flusher := NewDNSFlusher(FlushMethodAuto)

	method := flusher.detectMethod()

	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, FlushMethodBoth, method)
	case "linux":
		// Could be systemd, nscd, or auto depending on system
		assert.Contains(t, []FlushMethod{FlushMethodSystemd, FlushMethodNscd, FlushMethodAuto}, method)
	}
}

func TestFlushMethod_String(t *testing.T) {
	methods := map[FlushMethod]string{
		FlushMethodAuto:        "auto",
		FlushMethodDscacheutil: "dscacheutil",
		FlushMethodKillall:     "killall",
		FlushMethodBoth:        "both",
		FlushMethodSystemd:     "systemd",
		FlushMethodNscd:        "nscd",
	}

	for method, expected := range methods {
		t.Run(expected, func(t *testing.T) {
			assert.Equal(t, expected, string(method))
		})
	}
}

// Note: Actually testing DNS flush requires root and modifies system state,
// so we skip those tests in unit tests. They would be integration tests.

func TestDNSFlusher_Flush_UnsupportedOS(t *testing.T) {
	// This test only makes sense if we're not on darwin or linux
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		t.Skip("Test only applicable on unsupported OS")
	}

	flusher := NewDNSFlusher(FlushMethodAuto)
	err := flusher.Flush()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported operating system")
}

// Matrix test for flush methods
func TestFlushMethod_Matrix(t *testing.T) {
	methods := []FlushMethod{
		FlushMethodAuto,
		FlushMethodDscacheutil,
		FlushMethodKillall,
		FlushMethodBoth,
		FlushMethodSystemd,
		FlushMethodNscd,
	}

	platforms := []string{"darwin", "linux"}

	for _, method := range methods {
		for _, platform := range platforms {
			t.Run(string(method)+"_"+platform, func(t *testing.T) {
				flusher := NewDNSFlusher(method)
				assert.NotNil(t, flusher)

				// Just verify no panic when checking method
				_ = flusher.method
			})
		}
	}
}

func BenchmarkDNSFlusher_DetectMethod(b *testing.B) {
	flusher := NewDNSFlusher(FlushMethodAuto)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = flusher.detectMethod()
	}
}
