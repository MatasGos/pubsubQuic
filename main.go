package main

import (
	"context"
	"crypto/tls"
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
				log.Println("Failed to accept stream: ", err)
			}

			a.AddPublisher(0, conn.Context().Done())
			informPublishers(a)

			buf := make([]byte, 1024)
			for {
				n, err := stream.Read(buf)
				if err != nil {
					log.Println("Error when reading stream: ", err)
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
				log.Println("Failed to accept stream: ", err)
			}

			ch := a.AddSubscriber(0, conn.Context().Done())

			context.AfterFunc(conn.Context(), func() { informPublishers(a) })

			for message := range ch {
				_, err := stream.Write([]byte(message))
				if err != nil {
					log.Println("Error when writing to stream: ", err)
				}
			}
		}(conn)
	}
}

// informPublishers informs publishers if there are no subscribers connected
func informPublishers(a *pubsub.Agent[messageType]) {
	if a.ConnectedSubscribers() == 0 {
		a.NotifyPublishers("Currently there are no subscribers connected")
	}
}
