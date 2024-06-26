package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"math/big"
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
	listener, err := quic.ListenAddr(pubAddr, generateTLSConfig(), nil)
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
	listener, err := quic.ListenAddr(subAddr, generateTLSConfig(), nil)
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

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-pub-sub"},
	}
}
