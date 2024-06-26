package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"pubsubQUIC/config"
	"pubsubQUIC/pubsub"
)

const pubAddr = "localhost:5000" //TODO ENV
const subAddr = "localhost:4000"

type messageType string

func main() {
	agent := pubsub.NewAgent[messageType]()

	go func() { log.Fatal(publisherServer(agent)) }()
	go func() { log.Fatal(subscriberServer(agent)) }()
	go agent.CloseConnections()
}

func publisherServer(a *pubsub.Agent[messageType]) error {
	listener, err := quic.ListenAddr(pubAddr, config.GenerateTLSConfig(), nil)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}

		go func(conn quic.Connection) {
			stream, err := conn.AcceptStream(conn.Context())
			if err != nil {
				panic(err) //TODO handle panic
			}

			a.AddPublisher(0, conn.Context().Done())

			buf := make([]byte, 1024)
			for {
				n, err := stream.Read(buf)
				if err != nil {
					fmt.Println("Error when reading stream: ", err) //TODO logging
				}

				message := messageType(buf[:n])
				a.BroadcastEvent(conn.Context(), message)
			}
		}(conn)
	}
}

func subscriberServer(a *pubsub.Agent[messageType]) error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-pub-sub"},
	}

	listener, err := quic.ListenAddr(subAddr, tlsConf, nil)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}

		go func(conn quic.Connection) {
			stream, err := conn.AcceptStream(conn.Context())
			if err != nil {
				panic(err) //TODO handle panic
			}

			ch := a.AddSubscriber(0, conn.Context().Done())

			for message := range ch {
				_, err := stream.Write([]byte(message))
				if err != nil {
					fmt.Println("Error when writing stream: ", err) //TODO logging
				}
			}
		}(conn)
	}
}
