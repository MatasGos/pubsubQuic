package main

import (
	"github.com/joho/godotenv"
	"log"
	"pubsubQUIC/pubsub"
)

type messageType string

func main() {
	onRun()

	agent := pubsub.NewAgent[messageType](nil)

	go publisherServer(agent)
	go subscriberServer(agent)
	go agent.CloseConnections()

	//block in main thread forever
	select {}
}

func onRun() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}
