package main

import (
	"fmt"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
)

// Reason explains why the sliding window has made a decision.
type Reason uint8

const (
	ReasonFirst Reason = iota
	ReasonReuse
	ReasonShift
	ReasonOutOfWindow
)

func (r Reason) String() string {
	switch r {
	case ReasonFirst:
		return "First"
	case ReasonReuse:
		return "Reuse"
	case ReasonShift:
		return "Shift"
	case ReasonOutOfWindow:
		return "Small"
	}
	return "Unknown"
}

// Int256 is a simple 256 bit integer type.
type Int256 [4]uint64

// shiftLeft bit-shifts i by a bits to the left.
func shiftLeft(i Int256, a uint64) Int256 {
	// Note: Not branch optimized. Idiomatic code commented for clarity.
	// shift full words
	switch a / 64 {
	case 0:
		break
	case 1:
		//i[0], i[1], i[2], i[3] = i[1], i[2], i[3], 0
		i[0] = i[1]
		i[1] = i[2]
		i[2] = i[3]
		i[3] = 0
	case 2:
		//i[0], i[1], i[2], i[3] =  i[2], i[3], 0, 0
		i[0] = i[2]
		i[1] = i[3]
		i[2] = 0
		i[3] = 0
	case 3:
		//i[0], i[1], i[2], i[3] =  i[3], 0, 0, 0
		i[0] = i[3]
		i[1] = 0
		i[2] = 0
		i[3] = 0
	default:
		//i[0], i[1], i[2], i[3] = 0, 0, 0, 0
		i[0] = 0
		i[1] = 0
		i[2] = 0
		i[3] = 0
	}
	// shift remaining bits
	b := a % 64
	i[0] = (i[0] << b) | (i[1] >> (64 - b))
	i[1] = (i[1] << b) | (i[2] >> (64 - b))
	i[2] = (i[2] << b) | (i[3] >> (64 - b))
	i[3] = i[3] << b
	return i
}

// setBit sets bit number a in i. Count starts at 0.
func setBit(i Int256, a uint8) Int256 {
	i[a/64] = i[a/64] | 0x01<<(63-(a%64))
	return i
}

// isBitSet returns true if bit number a is true in i. Count starts at 0.
func isBitSet(i Int256, a uint8) bool {
	return i[a/64]&(0x01<<(63-(a%64))) != 0
}

// SlidingWindow implements a sliding window algorithm.
type SlidingWindow struct {
	offset uint64
	bitmap Int256
}

// CheckAndSetNonce returns true if the nonce is valid, false otherwise. It updates the SlidingWindow to prevent the nonce
// from being valid in the future.
func (window *SlidingWindow) CheckAndSetNonce(nonce uint64) (Reason, bool) {
	const windowSize = 256
	// Is the nonce on the left of the window and hence invalid?
	if nonce < window.offset {
		return ReasonOutOfWindow, false
	}
	// Is the nonce on the right of the window? If yes, shift window and update offset.
	if nonce >= window.offset+windowSize {
		newOffset := nonce - windowSize + 1
		shift := newOffset - window.offset
		window.offset = newOffset
		window.bitmap = shiftLeft(window.bitmap, shift)
		window.bitmap = setBit(window.bitmap, uint8(nonce-window.offset))
		return ReasonShift, true
	}
	// Nonce is within the window.
	bitPos := uint8(nonce - window.offset)
	if isBitSet(window.bitmap, bitPos) {
		return ReasonReuse, false
	}
	window.bitmap = setBit(window.bitmap, bitPos)
	return ReasonFirst, true
}

// CheckNonce returns true if the nonce is valid. It does not change the state.
func (window *SlidingWindow) CheckNonce(nonce uint64) (Reason, bool) {
	const windowSize = 256
	// Is the nonce on the left of the window and hence invalid?
	if nonce < window.offset {
		return ReasonOutOfWindow, false
	}
	// Is the nonce on the right of the window? If yes, shift window and update offset.
	if nonce >= window.offset+windowSize {
		return ReasonShift, true
	}
	// Nonce is within the window.
	bitPos := uint8(nonce - window.offset)
	if isBitSet(window.bitmap, bitPos) {
		return ReasonReuse, false
	}
	return ReasonFirst, true
}

func main() {
	nonces := argsToInt()
	if len(nonces) == 0 {
		fmt.Printf("Usage:\n$ %s <nonce> <nonce> <nonce>...\n\n", path.Base(os.Args[0]))
		os.Exit(1)
	}
	window := new(SlidingWindow)
	fmt.Println("\nApplying nonces in order:", nonces)
	fmt.Println("Nonce\tOK?\tReason\tOffset\tBitmap")
	fmt.Println(strings.Repeat("=", 288))
	for _, nonce := range nonces {
		reason, ok := window.CheckAndSetNonce(nonce)
		fmt.Printf("%d\t%t\t%s\t%d\t%s\n", nonce, ok, reason, window.offset, printWindow(window, nonce))
	}
}

// IGNORE BELOW: ======================================================================

// convert arguments to uint64
func argsToInt() []uint64 {
	if len(os.Args) < 2 {
		return nil
	}
	r := make([]uint64, len(os.Args)-1)
	j := 0
	for i := 1; i < len(os.Args); i++ {
		x, err := strconv.ParseUint(os.Args[i], 10, 64)
		if err != nil {
			continue
		}
		r[j] = x
		j++
	}
	return r
}

func blurString(s string, bitPos int) string {
	var one, zero, red, end = []byte("\u001B[0;37m"), []byte("\u001B[1;30m"), []byte("\033[0;31m"), []byte("\033[0m")
	var last byte
	color := func(b byte) []byte {
		switch b {
		case '1':
			return one
		case '0':
			return zero
		}
		return []byte{}
	}
	a := make([]byte, 0, len(s)+4)
	last = s[0]
	a = append(a, color(last)...)
	for p, b := range []byte(s) {
		if p == bitPos {
			a = append(a, end...)
			a = append(a, red...)
			a = append(a, b)
			a = append(a, end...)
			last = 0x00
			continue
		}
		if b != last {
			a = append(a, end...)
			a = append(a, color(b)...)
			last = b
		}
		a = append(a, b)
	}
	a = append(a, end...)
	return string(a)
}

// print state of the window, highlighting the bit to be tested/set
func printWindow(window *SlidingWindow, nonce uint64) string {
	if nonce < window.offset {
		return blurString(fmt.Sprintf("%.64b%.64b%.64b%.64b", window.bitmap[0], window.bitmap[1], window.bitmap[2], window.bitmap[3]), math.MaxInt)
	}
	return blurString(fmt.Sprintf("%.64b%.64b%.64b%.64b", window.bitmap[0], window.bitmap[1], window.bitmap[2], window.bitmap[3]), int(nonce-window.offset))
}
