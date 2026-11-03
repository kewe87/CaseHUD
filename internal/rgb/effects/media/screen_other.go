//go:build !windows

package media

func openScreen(display int) (grabber, error) { return nil, ErrUnsupported }
