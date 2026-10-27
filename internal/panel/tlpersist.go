package panel

// The only two persistent writes of the UNI FAN TL LCD driver besides the
// settings (op 64): the start image (op 0x48) and a serial number for a fan
// that has none (op 0x3F). Both live here, behind dedicated functions; there
// is no generic "send an operation" path. The deny-list test allows these two
// operations only through WriteStartImage, WriteRestImage and assignSerialIfEmpty.
//
// The fan compares what it is given with what it stores and only writes its
// flash when something differs, so both writes are made rarely: the host
// decides when (see internal/app/tlpersist.go), this file only does it safely.

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"sync"
	"time"
)

const (
	tlOpStartImage tlOp = 0x48 // start image, same framing as the stream frames, but every report is acknowledged
	tlOpRestImage  tlOp = 0x41 // stored picture (slot 1) shown when no host feeds the fan, same framing, every report acknowledged
	tlOpSetSerial  tlOp = 0x3F // 32 bytes of serial number, never answered

	tlSerialLen = 32
	tlAckWait   = tlSlow // wait for the acknowledgement of one start-image report
)

// Pause before the one retry of the settings restore after a start image
// (variable for tests).
var tlImageRetry = 300 * time.Millisecond

// ErrImageWritten is wrapped by the error of WriteStartImage when the image was
// completely written and acknowledged but the settings that follow were not
// confirmed: the image must be recorded as written, not sent again.
var ErrImageWritten = errors.New("start image written")

// StartImager is implemented by panels that keep a start image of their own.
type StartImager interface {
	Panel
	// SettingsKey identifies the device (serial number, else chain position).
	SettingsKey() string
	// WriteStartImage stores a 400x400 JPEG (at most 65535 bytes) as the
	// picture the device shows when no host is feeding it. This is a
	// permanent write: call it rarely.
	WriteStartImage(jpeg []byte) error
}

// RestImager is implemented by panels that also keep a stored picture they
// show after the host stops feeding them (slot 1, op 0x41). Close switches to it.
type RestImager interface {
	// WriteRestImage stores a 400x400 JPEG (at most 65535 bytes) in the picture
	// slot. Permanent write, same rules as WriteStartImage.
	WriteRestImage(jpeg []byte) error
}

// ExitImager is implemented by panels that can leave a picture on the display
// when Close runs. Nothing is stored: it is an ordinary frame sent last.
// The size is the panel's own landscape size or its upright portrait one.
type ExitImager interface {
	SetExitImage(img image.Image)
}

// WriteStartImage sends the picture as a series of numbered reports, reads the
// acknowledgement after every one and gives up at the first missing or wrong
// one. The chain is held for the whole transfer, so no frame of the live
// stream (nor another fan's traffic) interleaves; the stream of this fan
// resumes afterwards with the normal settings.
func (p *tlLCD) WriteStartImage(data []byte) error {
	return p.upload(tlOpStartImage, "start image", data)
}

// WriteRestImage stores the picture in slot 1 (op 0x41): the one the fan shows
// once the host stops streaming. Same transfer rules as WriteStartImage.
func (p *tlLCD) WriteRestImage(data []byte) error {
	return p.upload(tlOpRestImage, "stored image", data)
}

func (p *tlLCD) upload(op tlOp, what string, data []byte) error {
	if len(data) == 0 || len(data) > tlImageMax {
		return fmt.Errorf("TL LCD: %s size %d out of range", what, len(data))
	}
	tlChain.enter()
	defer tlChain.leave()
	if p.closed {
		return errTLClosed
	}
	werr := p.sendUpload(op, data)
	// Whatever happened (also an aborted transfer), the upload slot is freed
	// first: until the version query the fan ignores settings and stream. Then
	// the normal streaming settings go back so the panel keeps working (values
	// equal to what the fan holds change nothing); the restore is retried once.
	// The image is never sent again here. The stream check runs at the next frame.
	p.release()
	p.checkAt, p.watch = time.Time{}, true
	err := p.push(tlViewLive, true)
	if err != nil && werr == nil {
		time.Sleep(tlImageRetry)
		err = p.push(tlViewLive, true)
	}
	if err != nil && werr == nil {
		werr = fmt.Errorf("%w: TL LCD: settings after the %s: %w", ErrImageWritten, what, err)
	}
	return werr
}

func (p *tlLCD) sendUpload(op tlOp, data []byte) error {
	p.flush()
	buf := make([]byte, tlFrameSize)
	for seq, off := 0, 0; off < len(data); seq, off = seq+1, off+tlBodyMax {
		frame := tlFrame(op, len(data), seq, data[off:min(off+tlBodyMax, len(data))])
		if _, err := p.dev.Write(frame); err != nil {
			return fmt.Errorf("TL LCD: upload %#x report %d: %w", byte(op), seq, err)
		}
		n, err := p.dev.Read(buf, int(tlAckWait/time.Millisecond))
		if err != nil {
			return fmt.Errorf("TL LCD: upload %#x report %d: no acknowledgement: %w", byte(op), seq, err)
		}
		if n < tlHeadSize || buf[0] != tlReport || tlOp(buf[1]) != op {
			return fmt.Errorf("TL LCD: upload %#x report %d: unexpected acknowledgement", byte(op), seq)
		}
	}
	return nil
}

// idSerial is the serial number as far as IDs are concerned: one written by
// CaseHUD this run does not count, so panel IDs stay what they were.
func (p *tlLCD) idSerial() string {
	if p.serialNew {
		return ""
	}
	return p.serial
}

var (
	tlSerialRand io.Reader = rand.Reader
	tlSerialMu   sync.Mutex
	tlSerialDone = map[string]bool{} // USB paths a serial was tried on in this run
)

// newTLSerial returns the first 32 characters of a random lower-case UUID
// (version 4) string with dashes.
func newTLSerial() (string, error) {
	var u [16]byte
	if _, err := io.ReadFull(tlSerialRand, u[:]); err != nil {
		return "", err
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	s := fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
	return s[:tlSerialLen], nil
}

// assignSerialIfEmpty gives a fan without a serial number one. raw is the
// payload of the identity answer: the serial is empty when its 32 bytes are
// all zero. A serial that exists is never touched. The write is tried at most
// once per device and run and verified by reading the identity again. The
// caller holds the chain.
func (p *tlLCD) assignSerialIfEmpty(raw []byte) {
	if len(raw) < tlSerialLen || !bytes.Equal(raw[:tlSerialLen], make([]byte, tlSerialLen)) {
		return
	}
	regMu.RLock()
	off := opts.NoSerial
	regMu.RUnlock()
	if off {
		return
	}
	// Once per device and run. Without a USB path the chain position stands in
	// for it; with neither, nothing is written (it could not be deduplicated).
	key := p.path
	if key == "" && p.posKnown {
		key = "pos:" + tlPosID(p.port, p.slot)
	}
	if key == "" {
		log.Println("TL LCD: no serial number written: the display cannot be told apart from others")
		return
	}
	tlSerialMu.Lock()
	done := tlSerialDone[key]
	tlSerialDone[key] = true
	tlSerialMu.Unlock()
	if done {
		return
	}
	serial, err := newTLSerial()
	if err != nil {
		log.Println("TL LCD: no serial number written:", err)
		return
	}
	if _, err := p.dev.Write(tlFrame(tlOpSetSerial, tlSerialLen, 0, []byte(serial))); err != nil {
		log.Println("TL LCD: writing the serial number failed:", err)
		return
	}
	ans, err := p.request(tlOpIdentity, nil, tlSlow)
	if err != nil || len(ans) < tlHeadSize+tlSerialLen || string(ans[tlHeadSize:tlHeadSize+tlSerialLen]) != serial {
		log.Println("TL LCD: serial number written but not confirmed by the device")
		return
	}
	p.serial, p.serialNew = serial, true
	log.Println("TL LCD: serial number set on a display that had none:", serial)
}

// ErrClosed is returned by WriteStartImage when the panel was closed before
// anything was sent (a rescan): the write never reached the device.
var ErrClosed = errTLClosed
