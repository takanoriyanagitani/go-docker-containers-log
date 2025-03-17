package logs4containers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types/container"
	dc "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func LinesChanWriter(
	ctx context.Context,
	containerId string,
) (lines <-chan string, w io.Writer) {
	var ch chan string = make(chan string)
	rdr, wtr := io.Pipe()
	go func() {
		defer close(ch)
		var s *bufio.Scanner = bufio.NewScanner(rdr)
		for s.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var line string = s.Text()
			ch <- containerId + ":" + line
		}
	}()
	return ch, wtr
}

type Client struct {
	*dc.Client
	container.LogsOptions
	ContainerID string
}

func (c Client) ContainerLogs(ctx context.Context) (io.ReadCloser, error) {
	return c.Client.ContainerLogs(ctx, c.ContainerID, c.LogsOptions)
}

func (c Client) DemuxLogs(
	ctx context.Context,
) (out <-chan string, err <-chan string, e error) {
	rcloser, e := c.ContainerLogs(ctx)
	if nil != e {
		return nil, nil, e
	}

	out, wout := LinesChanWriter(ctx, c.ContainerID)
	err, werr := LinesChanWriter(ctx, c.ContainerID)

	go func() {
		defer rcloser.Close()

		_, e := stdcopy.StdCopy(wout, werr, rcloser)
		if nil != e {
			log.Printf("%v\n", e)
		}
	}()

	return out, err, nil
}

type LogChan struct {
	Out <-chan string
	Err <-chan string
}

func AllLogsToChan(
	ctx context.Context,
	logs []LogChan,
) (out <-chan string, err <-chan string) {
	var och chan string = make(chan string)
	var ech chan string = make(chan string)

	var stopCh chan struct{} = make(chan struct{})

	go func() {
		for range 2 * len(logs) {
			<-stopCh
		}

		close(och)
		close(ech)
	}()

	for _, log := range logs {
		go func(iout <-chan string) {
			defer func() {
				stopCh <- struct{}{}
			}()

			for msg := range iout {
				select {
				case <-ctx.Done():
					return
				default:
				}

				och <- msg
			}
		}(log.Out)

		go func(ierr <-chan string) {
			defer func() {
				stopCh <- struct{}{}
			}()

			for msg := range ierr {
				select {
				case <-ctx.Done():
					return
				default:
				}
				ech <- msg
			}
		}(log.Err)
	}
	return och, ech
}

func AllLogsToStd(ctx context.Context, logs []LogChan) {
	var owtr *bufio.Writer = bufio.NewWriter(os.Stdout)
	var ewtr *bufio.Writer = bufio.NewWriter(os.Stderr)

	och, ech := AllLogsToChan(ctx, logs)

	go func() {
		for msg := range och {
			_, e := fmt.Fprintln(owtr, msg)
			e = errors.Join(e, owtr.Flush())
			if nil != e {
				log.Printf("%v\n", e)
				return
			}
		}
	}()

	func() {
		for msg := range ech {
			_, e := fmt.Fprintln(ewtr, msg)
			e = errors.Join(e, ewtr.Flush())
			if nil != e {
				log.Printf("%v\n", e)
				return
			}
		}
	}()
}

var DefaultLogOptions container.LogsOptions = container.LogsOptions{
	ShowStdout: true,
	ShowStderr: true,
	Since:      "",
	Until:      "",
	Timestamps: true,
	Follow:     true,
	Tail:       "10",
	Details:    false,
}

func ContainerIdsToLogsToStd(
	ctx context.Context,
	ids []string,
	lopts container.LogsOptions,
	opts ...dc.Opt,
) error {
	var clients []*dc.Client
	var logs []LogChan

	for _, id := range ids {
		e := func(i string) error {
			client, e := dc.NewClientWithOpts(opts...)
			if nil != e {
				panic(e)
			}

			clients = append(clients, client)

			cli := Client{
				Client:      client,
				LogsOptions: lopts,
				ContainerID: i,
			}

			och, ech, e := cli.DemuxLogs(ctx)
			if nil != e {
				return e
			}

			lch := LogChan{
				Out: och,
				Err: ech,
			}

			logs = append(logs, lch)

			return nil
		}(id)

		if nil != e {
			return e
		}
	}

	defer func() {
		for _, cli := range clients {
			cli.Close()
		}
	}()

	AllLogsToStd(ctx, logs)

	return nil
}

func ContainerIdsToLogsToStdDefault(
	ctx context.Context,
	ids []string,
	opts ...dc.Opt,
) error {
	return ContainerIdsToLogsToStd(ctx, ids, DefaultLogOptions, opts...)
}
