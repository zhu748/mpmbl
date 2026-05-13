package protocol

import (
	"bufio"
	"errors"
	"io"
	"net/http"
)

func ScanSSELines(resp *http.Response, onLine func([]byte) bool) error {
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = trimLineBreak(line)
			if !onLine(line) {
				return nil
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func trimLineBreak(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line
}
