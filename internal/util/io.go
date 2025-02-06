package util

import "io"

func ReadMax(reader io.Reader, maxBytes int) ([]byte, bool, error) {
	buf := make([]byte, maxBytes)

	alreadyRead := 0
	for {
		n, err := reader.Read(buf[alreadyRead:])
		alreadyRead += n
		if alreadyRead == maxBytes {
			return buf, true, err
		}

		if err != nil {
			if err == io.EOF {
				return buf[:alreadyRead], false, nil
			} else {
				return buf[:alreadyRead], false, err
			}
		}
	}
}
