package tiktok

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// A minimal protobuf wire reader and writer. TikTok's webcast socket speaks
// protobuf; only a handful of fields matter here, so they are read by number
// instead of pulling in generated code and a protobuf runtime.

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5

	maxInflatedPayload = 16 << 20
)

type field struct {
	Num    int
	Wire   int
	Varint uint64
	Bytes  []byte
}

var errMalformed = errors.New("malformed protobuf")

func readFields(b []byte) ([]field, error) {
	var out []field

	for len(b) > 0 {
		tag, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, fmt.Errorf("%w: bad tag", errMalformed)
		}

		b = b[n:]

		f := field{Num: int(tag >> 3), Wire: int(tag & 7)}

		switch f.Wire {
		case wireVarint:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return nil, fmt.Errorf("%w: bad varint", errMalformed)
			}

			f.Varint = v
			b = b[n:]
		case wireFixed64:
			if len(b) < 8 {
				return nil, fmt.Errorf("%w: short fixed64", errMalformed)
			}

			f.Varint = binary.LittleEndian.Uint64(b)
			b = b[8:]
		case wireBytes:
			size, n := binary.Uvarint(b)
			if n <= 0 || size > uint64(len(b)-n) {
				return nil, fmt.Errorf("%w: bad length", errMalformed)
			}

			f.Bytes = b[n : n+int(size)]
			b = b[n+int(size):]
		case wireFixed32:
			if len(b) < 4 {
				return nil, fmt.Errorf("%w: short fixed32", errMalformed)
			}

			f.Varint = uint64(binary.LittleEndian.Uint32(b))
			b = b[4:]
		default:
			return nil, fmt.Errorf("%w: unsupported wire type %d", errMalformed, f.Wire)
		}

		out = append(out, f)
	}

	return out, nil
}

func appendVarintField(b []byte, num int, v uint64) []byte {
	b = binary.AppendUvarint(b, uint64(num)<<3|wireVarint)

	return binary.AppendUvarint(b, v)
}

func appendBytesField(b []byte, num int, v []byte) []byte {
	b = binary.AppendUvarint(b, uint64(num)<<3|wireBytes)
	b = binary.AppendUvarint(b, uint64(len(v)))

	return append(b, v...)
}

// pushFrame is the envelope of every binary frame on the socket.
type pushFrame struct {
	SeqID           uint64
	LogID           uint64
	PayloadEncoding string
	PayloadType     string
	Payload         []byte
}

func decodePushFrame(b []byte) (pushFrame, error) {
	fields, err := readFields(b)
	if err != nil {
		return pushFrame{}, err
	}

	var frame pushFrame

	for _, f := range fields {
		switch {
		case f.Num == 1 && f.Wire == wireVarint:
			frame.SeqID = f.Varint
		case f.Num == 2 && f.Wire == wireVarint:
			frame.LogID = f.Varint
		case f.Num == 6 && f.Wire == wireBytes:
			frame.PayloadEncoding = string(f.Bytes)
		case f.Num == 7 && f.Wire == wireBytes:
			frame.PayloadType = string(f.Bytes)
		case f.Num == 8 && f.Wire == wireBytes:
			frame.Payload = f.Bytes
		}
	}

	return frame, nil
}

func encodePushFrame(frame pushFrame) []byte {
	var b []byte

	if frame.SeqID != 0 {
		b = appendVarintField(b, 1, frame.SeqID)
	}

	if frame.LogID != 0 {
		b = appendVarintField(b, 2, frame.LogID)
	}

	b = appendBytesField(b, 6, []byte(frame.PayloadEncoding))
	b = appendBytesField(b, 7, []byte(frame.PayloadType))

	return appendBytesField(b, 8, frame.Payload)
}

type rawMessage struct {
	Method  string
	Payload []byte
	MsgID   uint64
}

// response is the body of a "msg" frame: a batch of events plus ack data.
type response struct {
	Messages    []rawMessage
	InternalExt []byte
	NeedsAck    bool
}

func decodeResponse(payload []byte) (response, error) {
	if len(payload) >= 2 && payload[0] == 0x1f && payload[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return response{}, fmt.Errorf("failed to open gzip payload: %w", err)
		}

		inflated, err := io.ReadAll(io.LimitReader(zr, maxInflatedPayload))
		if err != nil {
			return response{}, fmt.Errorf("failed to inflate payload: %w", err)
		}

		payload = inflated
	}

	fields, err := readFields(payload)
	if err != nil {
		return response{}, err
	}

	var resp response

	for _, f := range fields {
		switch {
		case f.Num == 1 && f.Wire == wireBytes:
			inner, err := readFields(f.Bytes)
			if err != nil {
				continue
			}

			var msg rawMessage

			for _, m := range inner {
				switch {
				case m.Num == 1 && m.Wire == wireBytes:
					msg.Method = string(m.Bytes)
				case m.Num == 2 && m.Wire == wireBytes:
					msg.Payload = m.Bytes
				case m.Num == 3 && m.Wire == wireVarint:
					msg.MsgID = m.Varint
				}
			}

			resp.Messages = append(resp.Messages, msg)
		case f.Num == 5 && f.Wire == wireBytes:
			resp.InternalExt = f.Bytes
		case f.Num == 9 && f.Wire == wireVarint:
			resp.NeedsAck = f.Varint != 0
		}
	}

	return resp, nil
}

func heartbeatFrame(roomID uint64) []byte {
	return encodePushFrame(pushFrame{
		PayloadEncoding: "pb",
		PayloadType:     "hb",
		Payload:         appendVarintField(nil, 1, roomID),
	})
}

func enterRoomFrame(roomID uint64) []byte {
	payload := appendVarintField(nil, 1, roomID)
	payload = appendVarintField(payload, 4, 12)
	payload = appendBytesField(payload, 5, []byte("audience"))
	payload = appendBytesField(payload, 9, []byte("0"))

	return encodePushFrame(pushFrame{
		PayloadEncoding: "pb",
		PayloadType:     "im_enter_room",
		Payload:         payload,
	})
}

func ackFrame(logID uint64, internalExt []byte) []byte {
	return encodePushFrame(pushFrame{
		LogID:           logID,
		PayloadEncoding: "pb",
		PayloadType:     "ack",
		Payload:         internalExt,
	})
}
