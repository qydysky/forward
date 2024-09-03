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

	"github.com/dustin/go-humanize"
	"github.com/qydysky/part"
	pctx "github.com/qydysky/part/ctx"
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
		// ctrl+c退出
		var interrupt = make(chan os.Signal, 2)
		signal.Notify(interrupt, os.Interrupt)

		ctx := pctx.CarryCancel(context.WithCancel(context.Background()))
		msdChan, wait := dealConfig(ctx, config)

		defer wait()
		defer func() {
			_ = pctx.CallCancel(ctx)
		}()

		for {
			select {
			case msg := <-msdChan:
				switch msg.fmsg.Type {
				case part.LisnMsg:
					log.Default().Printf("LISTEN %v => %v", msg.item.Listen, msg.item.To)
				case part.AcceptMsg:
					log.Default().Printf("ACCEPT %v => %v", (msg.fmsg.Msg).(net.Addr).String(), msg.item.To)
				case part.DenyMsg:
					log.Default().Printf("DENY   %v => %v", (msg.fmsg.Msg).(net.Addr).String(), msg.item.To)
				case part.ClosMsg:
					log.Default().Printf("CLOSE  %v => %v", (msg.fmsg.Msg).(net.Addr).String(), msg.item.To)
				case part.ErrorMsg:
					log.Default().Fatalf("ERROR %v => %v %v", msg.item.Listen, msg.item.To, msg.fmsg.Msg)
				default:
				}
			case <-interrupt:
				log.Default().Printf("CLOSE")
				return
			}
		}
	}
}

type ConfigMsg struct {
	item ConfigItem
	fmsg part.ForwardMsg
}

func dealConfig(ctx context.Context, config Config) (msgChan chan ConfigMsg, WaitFin func()) {
	msgChan = make(chan ConfigMsg, 10)
	var wg sync.WaitGroup
	wg.Add(len(config))
	for _, v := range config {
		go func(ctx context.Context, item ConfigItem) {
			defer wg.Done()

			var msg_chan chan part.ForwardMsg
			var close func()

			close, msg_chan = part.Forward(item.To, item.Listen, item.Accept)

			go func() {
				<-ctx.Done()
				close()
			}()
			defer func() {
				_ = pctx.CallCancel(ctx)
			}()

			for {
				select {
				case msg := <-msg_chan:
					select {
					case msgChan <- ConfigMsg{item: item, fmsg: msg}:
					default:
						<-msgChan
						msgChan <- ConfigMsg{item: item, fmsg: msg}
					}
					if msg.Type == part.ErrorMsg {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(ctx, v)
	}

	return msgChan, wg.Wait
}
