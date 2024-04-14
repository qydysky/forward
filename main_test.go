package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/qydysky/part"
	pctx "github.com/qydysky/part/ctx"
	preqf "github.com/qydysky/part/reqf"
	pweb "github.com/qydysky/part/web"
)

func Test(t *testing.T) {
	ctx := pctx.CarryCancel(context.WithCancel(context.Background()))
	msdChan, wait := dealConfig(ctx, []ConfigItem{
		{
			Listen: "tcp://127.0.0.1:20000",
			To:     "tcp://127.0.0.1:20001",
			Accept: []string{"127.0.0.1/32"},
		},
		{
			Listen: "tcp://127.0.0.1:20002",
			To:     "tcp://127.0.0.1:20003",
			Accept: []string{"127.0.0.2/32"},
		},
		{
			Listen: "udp://127.0.0.1:20000",
			To:     "udp://127.0.0.1:20001",
			Accept: []string{"127.0.0.1/32"},
		},
		{
			Listen: "udp://127.0.0.1:20004",
			To:     "udp://127.0.0.1:20005",
			Accept: []string{"127.0.0.2/32"},
		},
		{
			Listen: "udp://127.0.0.1:20006",
			To:     "tcp://127.0.0.1:20007",
			Accept: []string{"127.0.0.1/32"},
		},
		{
			Listen: "tcp://127.0.0.1:20008",
			To:     "udp://127.0.0.1:20009",
			Accept: []string{"127.0.0.1/32"},
		},
		{
			Listen: "tcp://127.0.0.1:20011",
			To:     "tcp://127.0.0.1:20010",
			Accept: []string{"127.0.0.1/32"},
		},
	})

	go func() {
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
				case part.ErrorMsg:
					log.Default().Fatalf("ERROR %v => %v %v", msg.item.Listen, msg.item.To, msg.fmsg.Msg)
				default:
				}
			case <-ctx.Done():
				log.Default().Printf("CLOSE")
				return
			}
		}
	}()
	defer wait()
	defer func() {
		_ = pctx.CallCancel(ctx)
	}()

	time.Sleep(time.Second)
	for i := 0; i < 100; i++ {
		if e := tcpSer("127.0.0.1:20000", "127.0.0.1:20001"); e != nil {
			t.Fatal(e)
		}
	}
	if e := tcpSer("127.0.0.1:20002", "127.0.0.1:20003"); e == nil {
		t.Fatal(e)
	}
	if e := udpSer("127.0.0.1:20000", "127.0.0.1:20001"); e != nil {
		t.Fatal(e)
	}
	if e := udpSer("127.0.0.1:20004", "127.0.0.1:20005"); e == nil {
		t.Fatal(e)
	}
	if e := udp2tcpSer("127.0.0.1:20006", "127.0.0.1:20007"); e != nil {
		t.Fatal(e)
	}
	if e := tcp2udpSer("127.0.0.1:20008", "127.0.0.1:20009"); e != nil {
		t.Fatal(e)
	}
	if e := tcp2udpSer("127.0.0.1:20008", "127.0.0.1:20009"); e != nil {
		t.Fatal(e)
	}
	if e := webSer("127.0.0.1:20008", "127.0.0.1:20009"); e != nil {
		t.Fatal(e)
	}
}

func tcpSer(lis, to string) error {
	ec := make(chan error, 10)
	{
		// Resolve the string address to a TCP address
		tcpAddr, err := net.ResolveTCPAddr("tcp4", to)

		if err != nil {
			return err
		}

		// Start listening for TCP connections on the given address
		listener, err := net.ListenTCP("tcp", tcpAddr)

		if err != nil {
			return err
		}

		defer listener.Close()

		go func() {
			// Accept new connections
			conn, err := listener.Accept()
			if err != nil {
				ec <- err
				return
			}
			defer conn.Close()
			// Handle new connections in a Goroutine for concurrency
			// Read from the connection untill a new line is send
			_, err = bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				ec <- err
				return
			}

			// Print the data read from the connection to the terminal

			// Write back the same message to the client
			_, _ = conn.Write([]byte("Hello TCP Client\n"))
		}()
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp4", lis)

	if err != nil {
		return err
	}

	// Connect to the address with tcp
	conn, err := net.DialTCP("tcp", nil, tcpAddr)

	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(time.Second))
	// Send a message to the server
	_, err = conn.Write([]byte("Hello TCP Server\n"))
	if err != nil {
		return err
	}

	// Read from the connection untill a new line is send
	data, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}

	if string(data) != "Hello TCP Client\n" {
		return errors.New("no match:" + string(data))
	}

	select {
	case err := <-ec:
		return err
	default:
		return nil
	}
}

func udpSer(lis, to string) error {
	ec := make(chan error, 10)
	{
		// Resolve the string address to a TCP address
		udpAddr, err := net.ResolveUDPAddr("udp", to)

		if err != nil {
			return err
		}
		conn, err := net.ListenUDP("udp", udpAddr)

		if err != nil {
			return err
		}

		go func() {
			var buf [512]byte
			defer conn.Close()
			for {
				_, addr, err := conn.ReadFromUDP(buf[0:])
				if err != nil {
					ec <- err
					return
				}
				// Write back the message over UPD
				_, err = conn.WriteToUDP([]byte("Hello UDP Client\n"), addr)
				if err != nil {
					ec <- err
					return
				}
			}
		}()
	}

	udpAddr, err := net.ResolveUDPAddr("udp", lis)

	if err != nil {
		return err
	}

	conn1, err := net.ListenUDP("udp", nil)

	if err != nil {
		return err
	}

	_, _ = conn1.WriteToUDP([]byte("Hello UDP Server\n"), udpAddr)

	var buf [512]byte
	_ = conn1.SetDeadline(time.Now().Add(time.Second))
	n, _, err := conn1.ReadFromUDP(buf[0:])
	if err != nil {
		return err
	}

	if string(buf[:n]) != "Hello UDP Client\n" {
		return errors.New("no match:" + string(buf[:n]))
	}

	_, _ = conn1.WriteToUDP([]byte("Hello UDP Server\n"), udpAddr)
	_ = conn1.SetDeadline(time.Now().Add(time.Second))
	n, _, err = conn1.ReadFromUDP(buf[0:])
	if err != nil {
		return err
	}

	if string(buf[:n]) != "Hello UDP Client\n" {
		return errors.New("no match:" + string(buf[:n]))
	}

	select {
	case err := <-ec:
		return err
	default:
		return nil
	}
}

func udp2tcpSer(lis, to string) error {
	ec := make(chan error, 10)
	{
		// Resolve the string address to a TCP address
		tcpAddr, err := net.ResolveTCPAddr("tcp4", to)

		if err != nil {
			return err
		}

		// Start listening for TCP connections on the given address
		listener, err := net.ListenTCP("tcp", tcpAddr)

		if err != nil {
			return err
		}

		defer listener.Close()

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				ec <- err
				return
			}
			defer conn.Close()

			_, err = bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				ec <- err
				return
			}

			// Print the data read from the connection to the terminal

			// Write back the same message to the client
			_, _ = conn.Write([]byte("Hello TCP Client\n"))
		}()
	}

	udpAddr, err := net.ResolveUDPAddr("udp", lis)

	if err != nil {
		return err
	}

	conn1, err := net.ListenUDP("udp", nil)

	if err != nil {
		return err
	}

	_, _ = conn1.WriteToUDP([]byte("Hello UDP Server\n"), udpAddr)

	var buf [512]byte
	_ = conn1.SetDeadline(time.Now().Add(time.Second))
	n, _, err := conn1.ReadFromUDP(buf[0:])
	if err != nil {
		return err
	}

	if string(buf[:n]) != "Hello TCP Client\n" {
		return errors.New("no match:" + string(buf[:n]))
	}

	select {
	case err := <-ec:
		return err
	default:
		return nil
	}
}

func tcp2udpSer(lis, to string) error {
	ec := make(chan error, 10)
	{
		listener, err := part.NewUdpListener("udp", to)

		if err != nil {
			return err
		}

		defer listener.Close()

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				ec <- err
				return
			}
			defer conn.Close()

			_, err = bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				ec <- err
				return
			}

			// Print the data read from the connection to the terminal

			// Write back the same message to the client
			_, _ = conn.Write([]byte("Hello UDP Client\n"))
		}()
	}

	addr, err := net.ResolveTCPAddr("tcp4", lis)

	if err != nil {
		return err
	}

	conn, err := net.DialTCP("tcp", nil, addr)

	if err != nil {
		return err
	}

	// Send a message to the server
	_, err = conn.Write([]byte("Hello TCP Server\n"))
	if err != nil {
		return err
	}

	// Read from the connection untill a new line is send
	data, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}

	if string(data) != "Hello UDP Client\n" {
		return errors.New("no match:" + string(data))
	}

	select {
	case err := <-ec:
		return err
	default:
		return nil
	}
}

func webSer(lis, to string) error {
	{
		w := pweb.New(&http.Server{
			Addr: "127.0.0.1:20010",
		})
		defer w.Shutdown()

		w.Handle(map[string]func(http.ResponseWriter, *http.Request){
			`/`: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("999"))
			},
		})
	}
	r := preqf.New()
	e := r.Reqf(preqf.Rval{
		Url: "http://127.0.0.1:20010",
	})
	fmt.Println(r.Respon)
	return e
}
