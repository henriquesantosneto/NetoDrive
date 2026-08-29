package syncer

import (
	"fmt"
	"os"
)

func syncLog(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	if len(format) == 0 || format[len(format)-1] != '\n' {
		_, _ = os.Stderr.Write([]byte{'\n'})
	}
}
