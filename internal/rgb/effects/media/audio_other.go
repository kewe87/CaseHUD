//go:build !windows

package media

func platformAudio() error { return ErrUnsupported }

func openLoopback(push func([]float32, int)) (interface{ Close() error }, error) {
	return nil, ErrUnsupported
}
