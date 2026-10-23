package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

func Write(w io.Writer, message Message) error {
	if err := message.Validate(); err != nil { return err }
	data, err := json.Marshal(message)
	if err != nil { return err }
	_, err = fmt.Fprintf(w, "%d\n%s\n", len(data), data)
	return err
}

func Read(r io.Reader) (Message, error) {
	reader := bufio.NewReader(r)
	var length int
	if _, err := fmt.Fscanf(reader, "%d\n", &length); err != nil { return Message{}, err }
	if length <= 0 || length > 16*1024*1024 { return Message{}, fmt.Errorf("invalid frame length %d", length) }
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil { return Message{}, err }
	if _, err := reader.ReadByte(); err != nil { return Message{}, err }
	var message Message
	if err := json.Unmarshal(data, &message); err != nil { return Message{}, err }
	if err := message.Validate(); err != nil { return Message{}, err }
	return message, nil
}
