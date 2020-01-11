package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// SetupLogger enables logging at debug level, and adds timestamps to the
// log for this debugging
func SetupLogger(debug bool) {
	logFormatter := plainFormatter{debug: debug}
	if debug {
		log.SetLevel(log.DebugLevel)
	}
	log.SetFormatter(&logFormatter)
}

type plainFormatter struct {
	debug bool
}

func (f *plainFormatter) Format(entry *log.Entry) ([]byte, error) {
	prefix := ""
	if f.debug {
		prefix = fmt.Sprintf("[%s] ", time.Now().Format(time.RFC3339Nano))
	}
	return []byte(fmt.Sprintf("%s%s\n", prefix, entry.Message)), nil
}
