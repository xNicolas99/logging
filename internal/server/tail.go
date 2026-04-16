package server

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// tailLog reads the last n lines from the given file and writes them to out.
func tailLog(file *os.File, out io.Writer, maxLines int) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	size := stat.Size()
	if size == 0 {
		return nil
	}

	const chunkSize = 4096
	buffer := make([]byte, chunkSize)
	var linesFound int
	var pos int64 = size
	var startPos int64 = 0

	for pos > 0 {
		move := int64(chunkSize)
		if pos < move {
			move = pos
		}
		pos -= move

		_, err := file.Seek(pos, 0)
		if err != nil {
			break
		}

		n, err := file.Read(buffer[:move])
		if err != nil {
			break
		}

		for i := n - 1; i >= 0; i-- {
			if buffer[i] == '\n' {
				// Ignore trailing newline at the very end of file
				if pos+int64(i) == size-1 {
					continue
				}
				linesFound++
				if linesFound == maxLines {
					startPos = pos + int64(i) + 1
					break
				}
			}
		}
		if linesFound == maxLines {
			break
		}
	}

	_, err = file.Seek(startPos, 0)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fmt.Fprintln(out, scanner.Text())
	}
	return scanner.Err()
}
