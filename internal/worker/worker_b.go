package worker

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"time"
)

type actorB struct {
	data actorBData
}

func NewWorkerTypeB() Worker {
	return &actorB{}
}

type actorBData struct {
	pid      int
	priority int
}

func (d actorBData) Report() string {
	return fmt.Sprintf(
		"\n===============\nREPORT:\nPID: %d\nPRIORITY: %d\n===============\n",
		d.pid, d.priority,
	)
}

func (ab *actorB) Start(ctx context.Context) <-chan string {
	reportsChan := make(chan string)

	go func() {
		ticker := time.NewTicker(time.Second * 3)
		defer ticker.Stop()
		defer close(reportsChan)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updated, err := ab.update()
				if err != nil {
				}
				if updated {
					reportsChan <- ab.data.Report()
				}
			}
		}
	}()

	return reportsChan
}

func (ab *actorB) update() (bool, error) {
	pid := ab.getPID()

	priority, err := ab.getProcessPriority()
	if err != nil {
		return false, fmt.Errorf("get process priority: %w", err)
	}

	updated := ab.data.pid != pid || ab.data.priority != priority

	if updated {
		ab.data = actorBData{
			pid:      pid,
			priority: priority,
		}
	}

	return updated, nil
}

func (ab *actorB) getProcessPriority() (int, error) {
	pid := os.Getpid()
	priority, err := unix.Getpriority(unix.PRIO_PROCESS, pid)
	if err != nil {
		return -1, fmt.Errorf("get priority: %w", err)
	}

	return priority, nil
}

func (ab actorB) getPID() int {
	return os.Getpid()
}
