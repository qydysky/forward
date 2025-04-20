package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/qydysky/part"
)

func Test(t *testing.T) {
	ctx, cancle := context.WithCancel(context.Background())
	wait := dealConfig(ctx, []ConfigItem{
		{
			Listen: "tcp://127.0.0.1:20000",
			To:     "tcp://127.0.0.1:20001",
			Accept: []string{"127.0.0.2/32", "127.0.0.1/32"},
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
		// {
		// 	Listen: "udp://127.0.0.1:20006",
		// 	To:     "tcp://127.0.0.1:20007",
		// 	Accept: []string{"127.0.0.1/32"},
		// },
		// {
		// 	Listen: "tcp://127.0.0.1:20008",
		// 	To:     "udp://127.0.0.1:20009",
		// 	Accept: []string{"127.0.0.1/32"},
		// },
		// {
		// 	Listen: "tcp://127.0.0.1:20012",
		// 	To:     "udp://127.0.0.1:20013",
		// 	Accept: []string{"127.0.0.1/32"},
		// },
		// {
		// 	Listen: "udp://127.0.0.1:20014",
		// 	To:     "tcp://127.0.0.1:20012",
		// 	Accept: []string{"127.0.0.1/32"},
		// },
		{
			Listen: "tcp://127.0.0.1:20011",
			To:     "tcp://127.0.0.1:20010",
			Accept: []string{"127.0.0.1/32"},
		},
	})

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
	// if e := tcp2udpSer("127.0.0.1:20008", "127.0.0.1:20009"); e != nil {
	// 	t.Fatal(e)
	// }
	// if e := udp2tcpSer("127.0.0.1:20006", "127.0.0.1:20007"); e != nil {
	// 	t.Fatal(e)
	// }
	if e := u2t2u("127.0.0.1:20014", "127.0.0.1:20013"); e != nil {
		t.Fatal(e)
	}
	cancle()
	wait()
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
	_ = conn.SetDeadline(time.Now().Add(time.Second))
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
				n, addr, err := conn.ReadFromUDP(buf[0:])
				if err != nil {
					ec <- err
					return
				}
				if !bytes.Equal([]byte("Hello UDP Server\n"), buf[:n]) {
					ec <- errors.New("ser rev err data")
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

// func udp2tcpSer(lis, to string) error {
// 	ec := make(chan error, 10)
// 	{
// 		// Resolve the string address to a TCP address
// 		tcpAddr, err := net.ResolveTCPAddr("tcp4", to)

// 		if err != nil {
// 			return err
// 		}

// 		// Start listening for TCP connections on the given address
// 		listener, err := net.ListenTCP("tcp", tcpAddr)

// 		if err != nil {
// 			return err
// 		}

// 		defer listener.Close()

// 		go func() {
// 			conn, err := listener.Accept()
// 			if err != nil {
// 				ec <- err
// 				return
// 			}
// 			defer conn.Close()

// 			data, err := bufio.NewReader(conn).ReadBytes('\n')
// 			if err != nil {
// 				ec <- err
// 			} else if _, err := conn.Write(data); err != nil {
// 				ec <- err
// 			} else {
// 			}
// 		}()
// 	}

// 	conn1, err := net.Dial("udp", lis)

// 	if err != nil {
// 		return err
// 	}

// 	size := 20000
// 	data := genData(size)

// 	if n, err := conn1.Write(data); err != nil || n != size {
// 		return err
// 	}

// 	// Read from the connection untill a new line is send
// 	buf2 := make([]byte, 100000)
// 	n, _ := conn1.Read(buf2)
// 	if !bytes.Equal(data, buf2[:n]) {
// 		return errors.New("no match")
// 	}

// 	select {
// 	case err := <-ec:
// 		return err
// 	default:
// 		return nil
// 	}
// }

// func tcp2udpSer(lis, to string) error {
// 	ec := make(chan error, 10)
// 	{
// 		listener, err := part.NewUdpListener("udp", to)

// 		if err != nil {
// 			return err
// 		}

// 		defer listener.Close()

// 		go func() {
// 			conn, err := listener.Accept()
// 			if err != nil {
// 				ec <- err
// 				return
// 			}

// 			data := make([]byte, 10000)
// 			n, err := conn.Read(data)
// 			if err != nil {
// 				ec <- err
// 			} else {
// 				_, err := conn.Write(data[:n])
// 				if err != nil {
// 					ec <- err
// 				}
// 			}
// 		}()
// 	}

// 	addr, err := net.ResolveTCPAddr("tcp4", lis)

// 	if err != nil {
// 		return err
// 	}

// 	conn, err := net.DialTCP("tcp", nil, addr)

// 	if err != nil {
// 		return err
// 	}

// 	size := 6666
// 	data := genData(size)

// 	// Send a message to the server
// 	if n, err := conn.Write(data); err != nil || n != size {
// 		return err
// 	}

// 	// Read from the connection untill a new line is send
// 	data2, err := bufio.NewReader(conn).ReadBytes('\n')
// 	if err != nil {
// 		return err
// 	}

// 	if !bytes.Equal(data, data2) {
// 		return errors.New("no match")
// 	}

// 	select {
// 	case err := <-ec:
// 		return err
// 	default:
// 		return nil
// 	}
// }

func genData(size int) []byte {
	data := make([]byte, size)
	if _, e := rand.Read(data); e != nil {
		panic(e)
	}
	data = bytes.ReplaceAll(data, []byte{'\n'}, []byte{' '})
	data[size-1] = '\n'
	return data
}

func u2t2u(lis, to string) error {
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

			data := make([]byte, 10000)
			n, err := conn.Read(data)
			if err != nil {
				ec <- err
			} else {
				_, err := conn.Write(data[:n])
				if err != nil {
					ec <- err
				}
			}
		}()
	}

	conn1, err := net.Dial("udp", lis)

	if err != nil {
		return err
	}

	size := 8888
	data := genData(size)

	if n, err := conn1.Write(data); err != nil || n != size {
		return err
	}

	// Read from the connection untill a new line is send
	buf2 := make([]byte, 10000)
	n, _ := conn1.Read(buf2)
	if !bytes.Equal(data, buf2[:n]) {
		return errors.New("no match")
	}

	select {
	case err := <-ec:
		return err
	default:
		return nil
	}
}
