package comm

import (
	"io"
	"os"
	"strings"
)

func Tail(name string, cutLine int) string {
	var pageSize int64 = 8192
	buf := make([]byte, pageSize)
	f, err := os.OpenFile(name, os.O_RDONLY, os.ModePerm)
	if err == nil {
		defer f.Close()
		end, err := f.Seek(0, io.SeekEnd)
		if err == nil {
			var lines = make([]string, 0)
			if end < pageSize {
				_, err = f.Seek(0, io.SeekStart)
				if err == nil {
					n, err := f.Read(buf)
					if err == nil {
						lines = strings.Split(string(buf[:n]), "\n")
					}
				}
			} else {
				var i int64 = 1
				for {
					_, err = f.Seek(-i*pageSize, io.SeekEnd)
					if err == nil {
						n, err := f.Read(buf)
						if err == nil {
							_lines := strings.Split(string(buf[:n]), "\n")
							lines = append(_lines, lines...)
							if len(lines) >= cutLine || i*pageSize > end {
								break
							}
						} else {
							break
						}
					} else {
						break
					}
					i++
				}
			}
			startPos := 0
			if len(lines) > cutLine {
				startPos = len(lines) - cutLine
			}
			return strings.Join(lines[startPos:], "\n")
		}
	}
	return ""
}