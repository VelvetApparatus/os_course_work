package main

import (
	"context"
	"fmt"
	"os/signal"
	"os_coursach/config"
	"os_coursach/internal/server"
	"os_coursach/internal/worker"
	"syscall"
)

func main() {

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGKILL, syscall.SIGSTOP)
	defer cancel()

	err := config.Init()
	if err != nil {
		fmt.Printf("init config: %s", err.Error())
		return
	}

	serverA := server.New(config.C().SaSets.Host, config.C().SaSets.Port, config.C().SaSets.LogPath)
	err = serverA.RunWithWorker(ctx, worker.NewWorkerTypeA(), "serverA")
	if err != nil {
		fmt.Printf("run server A: %s", err.Error())
		return
	}

	serverB := server.New(config.C().SbSets.Host, config.C().SbSets.Port, config.C().SbSets.LogPath)
	err = serverB.RunWithWorker(ctx, worker.NewWorkerTypeB(), "serverB")
	if err != nil {
		fmt.Printf("run server B: %s", err.Error())
		return
	}

	<-ctx.Done()

}
