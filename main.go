package main

import (
	"context"
	"crypto/tls"
	"github.com/joho/godotenv"
	"github.com/quic-go/quic-go"
	"log"
	"os"
	"pubsubQUIC/config"
	"pubsubQUIC/pubsub"
)

type messageType string

func main() {
	onRun()

	agent := pubsub.NewAgent[messageType]()

	go publisherServer(agent)
	go subscriberServer(agent)

	//block in main thread forever
	select {}
}

func publisherServer(a *pubsub.Agent[messageType]) {
	listener, err := quic.ListenAddr(os.Getenv("PUBLISHER_ADDRESS"), config.GenerateTLSConfig(), nil)
	if err != nil {
		log.Fatal("Error when creating publisher server: ", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Fatal("Error when accepting connection: ", err)
		}

		go func(conn quic.Connection) {
			stream, err := conn.AcceptStream(conn.Context())
			if err != nil {
				log.Println("Failed to accept stream: ", err)
			}

			a.AddPublisher(0, conn.Context().Done())
			informPublishers(a)

			context.AfterFunc(conn.Context(), func() {
				stream.Close()
			})

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

func subscriberServer(a *pubsub.Agent[messageType]) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{os.Getenv("NEXTPROTOS")},
	}

	listener, err := quic.ListenAddr(os.Getenv("SUBSCRIBER_ADDRESS"), tlsConf, nil)
	if err != nil {
		log.Fatal("Error when creating subscriber server: ", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Fatal("Error when accepting connection: ", err)
		}

		go func(conn quic.Connection) {
			stream, err := conn.AcceptStream(conn.Context())
			if err != nil {
				log.Println("Failed to accept stream: ", err)
			}

			ch := a.AddSubscriber(0, conn.Context().Done())

			context.AfterFunc(conn.Context(), func() {
				informPublishers(a)
				stream.Close()
			})

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

func onRun() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}
