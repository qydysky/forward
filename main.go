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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
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
	dl    time.Duration
}

func (t *fm) ErrorMsg(targetaddr, listenaddr string, e error) {
	log.Default().Printf("ERROR %v => %v %v", listenaddr, targetaddr, pe.ErrorFormat(e, pe.ErrActionInLineFunc))
}
func (t *fm) WarnMsg(targetaddr, listenaddr string, e error) {
	// log.Default().Printf("Warn  %v => %v %v", listenaddr, targetaddr, pe.ErrorFormat(e, pe.ErrActionInLineFunc))
}
func (t *fm) AcceptMsg(remote net.Addr, targetaddr string) (close func(error)) {
	current := t.id.Add(1)
	if current >= 99 {
		t.id.Store(0)
	}

	log.Default().Printf("ACCEPT %d %d %v => %v", t.alive.Add(1), current, remote.Network()+"://"+remote.String(), targetaddr)
	return func(e error) {
		if errors.Is(e, io.EOF) {
			log.Default().Printf("CONFIN %d %d %v => %v", t.alive.Add(-1), current, remote.Network()+"://"+remote.String(), targetaddr)
		} else if errors.Is(e, net.ErrClosed) {
			log.Default().Printf("CONCLO %d %d %v => %v", t.alive.Add(-1), current, remote.Network()+"://"+remote.String(), targetaddr)
		} else if errors.Is(e, os.ErrDeadlineExceeded) {
			log.Default().Printf("CONEXP %d %d %v => %v", t.alive.Add(-1), current, remote.Network()+"://"+remote.String(), targetaddr)
		} else {
			log.Default().Printf("CONERR %d %d %v => %v %v", t.alive.Add(-1), current, remote.Network()+"://"+remote.String(), targetaddr, e)
		}
	}
}
func (t *fm) ConnWaitMsg(conn net.Conn) {
	if t.dl != 0 {
		_ = conn.SetDeadline(time.Now().Add(t.dl))
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
	for _, v := range config {
		go func(item ConfigItem) {
			defer wg.Done()

			fmp := &fm{}
			if d, e := time.ParseDuration(item.IdleDru); e == nil {
				fmp.dl = d
			}
			defer Forward(item.To, item.Listen, item.Accept, fmp)()

			<-ctx.Done()
		}(v)
	}
	return wg.Wait
}

var (
	ErrForwardAccept    pe.Action = `ErrForwardAccept`
	ErrForwardDail      pe.Action = `ErrForwardDail`
	ErrNetworkNoSupport           = errors.New("ErrNetworkNoSupport")
	ErrUdpOverflow                = errors.New("ErrUdpOverflow")
)

type ForwardMsgFunc interface {
	ErrorMsg(targetaddr, listenaddr string, e error)
	WarnMsg(targetaddr, listenaddr string, e error)
	AcceptMsg(remote net.Addr, targetaddr string) (ConFinMsg func(error))
	ConnWaitMsg(conn net.Conn)
	DenyMsg(remote net.Addr, targetaddr string)
	LisnMsg(targetaddr, listenaddr string)
	ClosMsg(targetaddr, listenaddr string)
}

func Forward(targetaddr, listenaddr string, acceptCIDRs []string, callBack ForwardMsgFunc) (closef func()) {
	closef = func() {}

	lisNet := strings.Split(listenaddr, "://")[0]
	lisAddr := strings.Split(listenaddr, "://")[1]
	tarNet := strings.Split(targetaddr, "://")[0]
	tarAddr := strings.Split(targetaddr, "://")[1]
	lisIsUdp := strings.Contains(lisNet, "udp")
	tarIsUdp := strings.Contains(tarNet, "udp")

	if (!lisIsUdp && tarIsUdp) || (lisIsUdp && !tarIsUdp) {
		callBack.ErrorMsg(targetaddr, listenaddr, ErrNetworkNoSupport)
		return
	}

	//尝试监听
	var listener net.Listener
	{
		var err error
		switch lisNet {
		case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
			listener, err = net.Listen(lisNet, lisAddr)
		case "udp", "udp4", "udp6":
			listener, err = NewUdpListener(lisNet, lisAddr)
		default:
			err = ErrNetworkNoSupport
		}
		if err != nil {
			callBack.ErrorMsg(targetaddr, listenaddr, err)
			return
		}
	}
	{
		var err error
		switch lisNet {
		case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6", "ip", "ip4", "ip6", "unix", "unixgram", "unixpacket":
		default:
			err = ErrNetworkNoSupport
		}
		if err != nil {
			callBack.ErrorMsg(targetaddr, listenaddr, err)
			return
		}
	}

	//初始化关闭方法
	closef = func() {
		listener.Close()
		callBack.ClosMsg(targetaddr, listenaddr)
	}

	//返回监听地址
	callBack.LisnMsg(targetaddr, listenaddr)

	matchfunc := []func(ip net.IP) bool{}
	for _, cidr := range acceptCIDRs {
		if _, cidrx, err := net.ParseCIDR(cidr); err != nil {
			callBack.ErrorMsg(targetaddr, listenaddr, err)
			return
		} else {
			matchfunc = append(matchfunc, cidrx.Contains)
		}
	}

	//开始准备转发
	go func(listener net.Listener) {
		defer listener.Close()

		for {
			proxyconn, err := listener.Accept()
			if errors.Is(err, ErrUdpConnected) {
				continue
			}
			if err != nil {
				//返回Accept错误
				callBack.WarnMsg(targetaddr, listenaddr, pe.Join(ErrForwardAccept, err))
				continue
			}

			host, _, err := net.SplitHostPort(proxyconn.RemoteAddr().String())
			if err != nil {
				callBack.WarnMsg(targetaddr, listenaddr, err)
				continue
			}

			ip := net.ParseIP(host)

			var accept bool
			for i := 0; !accept && i < len(matchfunc); i++ {
				accept = accept || matchfunc[i](ip)
			}
			if !accept {
				//返回Deny
				callBack.DenyMsg(proxyconn.RemoteAddr(), targetaddr)
				proxyconn.Close()
				continue
			}

			go func() {
				//返回Accept
				conFin := callBack.AcceptMsg(proxyconn.RemoteAddr(), targetaddr)

				targetconn, err := net.Dial(tarNet, tarAddr)
				if err != nil {
					callBack.WarnMsg(targetaddr, listenaddr, pe.Join(ErrForwardDail, err))
					return
				}

				var (
					bufSize = 65536
					wg      = make(chan error, 3)
				)

				buf := make([]byte, bufSize*2)

				go func() {
					buf := buf[:bufSize]
					for {
						callBack.ConnWaitMsg(proxyconn)
						if n, err := proxyconn.Read(buf); err != nil {
							wg <- err
							break
						} else if _, err = targetconn.Write(buf[:n]); err != nil {
							wg <- err
							break
						}
					}
				}()

				go func() {
					buf := buf[bufSize:]
					for {
						callBack.ConnWaitMsg(targetconn)
						if n, err := targetconn.Read(buf); err != nil {
							wg <- err
							break
						} else if _, err = proxyconn.Write(buf[:n]); err != nil {
							wg <- err
							break
						}
					}
				}()

				conFin(<-wg)
				proxyconn.Close()
				targetconn.Close()
			}()
		}
	}(listener)

	return
}
