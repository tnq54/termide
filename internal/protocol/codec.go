package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

func Write(w io.Writer, message Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%d\n%s\n", len(data), data)
	return err
}

func Read(r io.Reader) (Message, error) {
	// Read exactly one frame: a temporary buffered reader would consume
	// bytes from subsequent frames and discard them when this call returns.
	header := make([]byte, 0, 10)
	var b [1]byte
	for {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return Message{}, err
		}
		if b[0] == '\n' {
			break
		}
		if b[0] < '0' || b[0] > '9' || len(header) >= 10 {
			return Message{}, fmt.Errorf("invalid frame header")
		}
		header = append(header, b[0])
	}
	length, err := strconv.Atoi(string(header))
	if err != nil {
		return Message{}, err
	}
	if length <= 0 || length > 16*1024*1024 {
		return Message{}, fmt.Errorf("invalid frame length %d", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return Message{}, err
	}
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return Message{}, err
	}
	if b[0] != '\n' {
		return Message{}, fmt.Errorf("invalid frame terminator")
	}
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, err
	}
	if err := message.Validate(); err != nil {
		return Message{}, err
	}
	return message, nil
}
