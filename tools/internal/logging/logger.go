package logging

import (
	"io"
	"log"
	"os"
)

func New() *log.Logger {
	return log.New(io.Writer(os.Stderr), "saki-tools ", log.LstdFlags|log.Lmsgprefix)
}
