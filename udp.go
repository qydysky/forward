package main

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	psync "github.com/qydysky/part/sync"
)

var ErrUdpConnOverflow = errors.New(`ErrUdpConnOverflow`)

type udpConn struct {
	e         error
	conn      *net.UDPConn
	remoteAdd *net.UDPAddr
	ctx       context.Context
	ctxCancel context.CancelFunc
	buf       chan []byte
	closef    func() error
}

func (t *udpConn) SetBuf(b []byte) {
	tmp := make([]byte, len(b))
	copy(tmp, b)
	select {
	case t.buf <- tmp:
	default:
		t.e = ErrUdpConnOverflow
	}
}
func (t *udpConn) Read(b []byte) (n int, err error) {
	select {
	case tmp := <-t.buf:
		n = copy(b, tmp)
	case <-t.ctx.Done():
		err = os.ErrDeadlineExceeded
	}
	return
}
func (t *udpConn) Write(b []byte) (n int, err error) {
	select {
	case <-t.ctx.Done():
		err = os.ErrDeadlineExceeded
	default:
		n, err = t.conn.WriteToUDP(b, t.remoteAdd)
		if err != nil {
			t.ctx.Done()
		}
	}
	return
}
func (t *udpConn) Close() error {
	t.ctxCancel()
	return t.closef()
}
func (t *udpConn) LocalAddr() net.Addr  { return t.conn.LocalAddr() }
func (t *udpConn) RemoteAddr() net.Addr { return t.remoteAdd }
func (t *udpConn) SetDeadline(b time.Time) error {
	time.AfterFunc(time.Until(b), func() {
		t.Close()
	})
	return nil
}
func (t *udpConn) SetReadDeadline(b time.Time) error  { return t.SetDeadline(b) }
func (t *udpConn) SetWriteDeadline(b time.Time) error { return t.SetDeadline(b) }

type udpLis struct {
	udpAddr *net.UDPAddr
	c       <-chan *udpConn
	closef  func() error
}

var ErrUdpConnected error = errors.New("ErrUdpConnected")

func NewUdpListener(network, listenaddr string) (*udpLis, error) {
	udpAddr, err := net.ResolveUDPAddr(network, listenaddr)
	if err != nil {
		return nil, err
	}
	if conn, err := net.ListenUDP(network, udpAddr); err != nil {
		return nil, err
	} else {
		c := make(chan *udpConn, 10)
		lis := &udpLis{
			udpAddr: udpAddr,
			c:       c,
			closef: func() error {
				return conn.Close()
			},
		}
		go func() {
			var link psync.MapG[string, *udpConn]
			buf := make([]byte, humanize.MByte)
			for {
				n, remoteAdd, e := conn.ReadFromUDP(buf)
				if e != nil {
					c <- &udpConn{e: e}
					return
				}
				if udpc, ok := link.Load(remoteAdd.String()); ok {
					udpc.SetBuf(buf[:n])
					c <- &udpConn{e: ErrUdpConnected}
				} else {
					udpc := &udpConn{
						conn:      conn,
						remoteAdd: remoteAdd,
						closef: func() error {
							link.Delete(remoteAdd.String())
							return nil
						},
						buf: make(chan []byte, 5),
					}
					udpc.ctx, udpc.ctxCancel = context.WithTimeout(context.Background(), time.Second*30)
					udpc.SetBuf(buf[:n])
					link.Store(remoteAdd.String(), udpc)
					c <- udpc
				}
			}
		}()
		return lis, nil
	}
}
func (t *udpLis) Accept() (net.Conn, error) {
	udpc := <-t.c
	return udpc, udpc.e
}
func (t *udpLis) Close() error {
	return t.closef()
}
func (t *udpLis) Addr() net.Addr {
	return t.udpAddr
}
