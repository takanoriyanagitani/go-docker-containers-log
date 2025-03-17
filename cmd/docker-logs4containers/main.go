package main

import (
	"context"
	"log"
	"os"

	dc "github.com/docker/docker/client"
	l4c "github.com/takanoriyanagitani/go-docker-containers-log"
)

var containerIds []string = os.Args[1:]

var opts []dc.Opt = []dc.Opt{
	dc.FromEnv,
	dc.WithAPIVersionNegotiation(),
}

func main() {
	e := l4c.ContainerIdsToLogsToStdDefault(
		context.Background(),
		containerIds,
		opts...,
	)
	if nil != e {
		log.Printf("%v\n", e)
	}
}
