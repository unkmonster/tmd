package utils

import (
	"runtime"

	log "github.com/sirupsen/logrus"
)

func RecoverWithLog(name string) {
	if r := recover(); r != nil {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, false)
		log.Errorf("[%s] Panic recovered: %v\n%s", name, r, buf[:n])
	}
}
