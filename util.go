package pool

import "io"

type Stoppable interface {
	Stop()
}

func Close(c io.Closer) {
	if err := c.Close(); err != nil {
		_, _ = stdLogger.Write([]byte(err.Error()))
	}
}
