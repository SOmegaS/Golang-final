package main

import (
	"context"
	"log"
	"tmp/pkg/kafka"
)

const mess = "{\n    \"id\": \"770a2336-f356-49b9-934d-e2da8bf02059\",\n    \"source\": \"/driver\",\n    \"type\": \"trip.command.end\",\n    \"datacontenttype\": \"application/json\",\n    \"time\": \"2023-11-09T17:31:00Z\",\n    \"data\": {\n        \"trip_id\": \"770a2336-f356-49b9-934d-e2da8bf02059\"\n    }\n}"

func main() {
	conn, _ := kafka.ConnectKafka(context.Background(), "kafka:9092", "driver-client-trip-topic", 0)
	err := kafka.SendToTopic(conn, []byte(mess))
	if err != nil {
		log.Fatal(err)
	}
}
