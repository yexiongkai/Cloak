package multiplex

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"
)

type Obfser func(*Frame) ([]byte, error)
type Deobfser func([]byte) (*Frame, error)

var u32 = binary.BigEndian.Uint32

const headerLen = 12

// For each frame, the three parts of the header is xored with three keys.
// The keys are generated from the SID and the payload of the frame.
func genXorKeys(key, nonce []byte) (i uint32, ii uint32, iii uint8) {
	h := sha1.New()
	hashed := h.Sum(append(key, nonce...))
	return u32(hashed[0:4]), u32(hashed[4:8]), hashed[8]
}

func MakeObfs(key []byte) Obfser {
	obfs := func(f *Frame) ([]byte, error) {
		obfsedHeader := make([]byte, headerLen)
		// header: [StreamID 4 bytes][Seq 4 bytes][Closing 1 byte][Nonce 3 bytes]
		io.ReadFull(rand.Reader, obfsedHeader[9:12])
		i, ii, iii := genXorKeys(key, obfsedHeader[9:12])
		binary.BigEndian.PutUint32(obfsedHeader[0:4], f.StreamID^i)
		binary.BigEndian.PutUint32(obfsedHeader[4:8], f.Seq^ii)
		obfsedHeader[8] = f.Closing ^ iii

		// Composing final obfsed message
		// We don't use util.AddRecordLayer here to avoid unnecessary malloc
		obfsed := make([]byte, 5+headerLen+len(f.Payload))
		obfsed[0] = 0x17
		obfsed[1] = 0x03
		obfsed[2] = 0x03
		binary.BigEndian.PutUint16(obfsed[3:5], uint16(headerLen+len(f.Payload)))
		copy(obfsed[5:5+headerLen], obfsedHeader)
		copy(obfsed[5+headerLen:], f.Payload)
		// obfsed: [record layer 5 bytes][cipherheader 12 bytes][payload]
		return obfsed, nil
	}
	return obfs
}

func MakeDeobfs(key []byte) Deobfser {
	deobfs := func(in []byte) (*Frame, error) {
		if len(in) < 5+headerLen {
			return nil, errors.New("Input cannot be shorter than 17 bytes")
		}
		peeled := in[5:]
		i, ii, iii := genXorKeys(key, peeled[9:12])
		streamID := u32(peeled[0:4]) ^ i
		seq := u32(peeled[4:8]) ^ ii
		closing := peeled[8] ^ iii
		payload := make([]byte, len(peeled)-headerLen)
		copy(payload, peeled[headerLen:])
		ret := &Frame{
			StreamID: streamID,
			Seq:      seq,
			Closing:  closing,
			Payload:  payload,
		}
		return ret, nil
	}
	return deobfs
}
