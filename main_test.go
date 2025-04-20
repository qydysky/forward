package main

import (
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
		// {
		// 	Listen: "tcp://127.0.0.1:20000",
		// 	To:     "tcp://127.0.0.1:20001",
		// 	Accept: []string{"127.0.0.2/32", "127.0.0.1/32"},
		// },
		// {
		// 	Listen: "tcp://127.0.0.1:20002",
		// 	To:     "tcp://127.0.0.1:20003",
		// 	Accept: []string{"127.0.0.2/32"},
		// },
		// {
		// 	Listen: "udp://127.0.0.1:20000",
		// 	To:     "udp://127.0.0.1:20001",
		// 	Accept: []string{"127.0.0.1/32"},
		// },
		// {
		// 	Listen: "udp://127.0.0.1:20004",
		// 	To:     "udp://127.0.0.1:20005",
		// 	Accept: []string{"127.0.0.2/32"},
		// },
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
		{
			Listen: "tcp://127.0.0.1:20012",
			To:     "udp://127.0.0.1:20013",
			Accept: []string{"127.0.0.1/32"},
		},
		{
			Listen: "udp://127.0.0.1:20014",
			To:     "tcp://127.0.0.1:20012",
			Accept: []string{"127.0.0.1/32"},
		},
		// {
		// 	Listen: "tcp://127.0.0.1:20011",
		// 	To:     "tcp://127.0.0.1:20010",
		// 	Accept: []string{"127.0.0.1/32"},
		// },
	})

	time.Sleep(time.Second)
	// for i := 0; i < 100; i++ {
	// 	if e := tcpSer("127.0.0.1:20000", "127.0.0.1:20001"); e != nil {
	// 		t.Fatal(e)
	// 	}
	// }
	// if e := tcpSer("127.0.0.1:20002", "127.0.0.1:20003"); e == nil {
	// 	t.Fatal(e)
	// }
	// if e := udpSer("127.0.0.1:20000", "127.0.0.1:20001"); e != nil {
	// 	t.Fatal(e)
	// }
	// if e := udpSer("127.0.0.1:20004", "127.0.0.1:20005"); e == nil {
	// 	t.Fatal(e)
	// }
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
