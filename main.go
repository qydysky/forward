package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"

	"github.com/dustin/go-humanize"
	"github.com/qydysky/part"
	pe "github.com/qydysky/part/errors"
	file "github.com/qydysky/part/file"
)

func main() {
	// 获取config路径
	c := flag.String("c", "main.json", "c")
	flag.Parse()
	if *c == "" {
		return
	}

	f := file.New(*c, 0, true)
	if !f.IsExist() {
		log.Fatal("config no exist")
		return
	}

	var config Config
	if data, e := f.ReadAll(humanize.KByte, humanize.MByte); e != nil && !errors.Is(e, io.EOF) {
		log.Fatal(e)
		return
	} else if e := json.Unmarshal(data, &config); e != nil {
		log.Fatal(e)
		return
	} else {
		ctx, cancle := context.WithCancel(context.Background())
		wait := dealConfig(ctx, config)
		// ctrl+c退出
		var interrupt = make(chan os.Signal, 2)
		signal.Notify(interrupt, os.Interrupt)
		<-interrupt
		cancle()
		wait()
	}
}

type fm struct {
	id    atomic.Uint32
	alive atomic.Int32
}

func (t *fm) ErrorMsg(targetaddr, listenaddr string, e error) {
	log.Default().Printf("ERROR %v => %v %v", listenaddr, targetaddr, pe.ErrorFormat(e, pe.ErrActionInLineFunc))
}
func (t *fm) WarnMsg(targetaddr, listenaddr string, e error) {
	// log.Default().Printf("Warn  %v => %v %v", listenaddr, targetaddr, pe.ErrorFormat(e, pe.ErrActionInLineFunc))
}
func (t *fm) AcceptMsg(remote net.Addr, targetaddr string) (close func()) {
	current := t.id.Add(1)
	if current >= 99 {
		t.id.Store(0)
	}

	log.Default().Printf("ACCEPT %d %d %v => %v", t.alive.Add(1), current, remote.Network()+"://"+remote.String(), targetaddr)
	return func() {
		log.Default().Printf("CONFIN %d %d %v => %v", t.alive.Add(-1), current, remote.Network()+"://"+remote.String(), targetaddr)
	}
}
func (t *fm) DenyMsg(remote net.Addr, targetaddr string) {
	current := t.id.Add(1)
	if current >= 99 {
		t.id.Store(0)
	}
	log.Default().Printf("DENY   %d %v => %v", current, remote.Network()+"://"+remote.String(), targetaddr)
}
func (t *fm) LisnMsg(targetaddr, listenaddr string) {
	log.Default().Printf("LISTEN %v => %v", listenaddr, targetaddr)
}
func (t *fm) ClosMsg(targetaddr, listenaddr string) {
	log.Default().Printf("CLOSE %v => %v", listenaddr, targetaddr)
}

func dealConfig(ctx context.Context, config Config) (WaitFin func()) {
	var wg sync.WaitGroup
	wg.Add(len(config))
	fmp := &fm{}
	for _, v := range config {
		go func(item ConfigItem) {
			defer wg.Done()

			defer part.Forward(item.To, item.Listen, item.Accept, fmp)()

			<-ctx.Done()
		}(v)
	}
	return wg.Wait
}
