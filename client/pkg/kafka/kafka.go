package kafka

import (
	"context"
	"github.com/segmentio/kafka-go"
)

// SendToTopic отправляет message в conn Kafka
func SendToTopic(conn *kafka.Conn, message []byte) error {
	_, err := conn.WriteMessages(
		kafka.Message{Value: message},
	)
	return err
}

// ReadFromTopic читает из conn Kafka
func ReadFromTopic(conn *kafka.Conn) ([]byte, error) {
	// Считываем 10КБ
	b := make([]byte, 10e3)
	n, err := conn.Read(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}

// ConnectKafka создает conn к Kafka
func ConnectKafka(ctx context.Context, address string, topic string, partition int) (*kafka.Conn, error) {
	return kafka.DialLeader(ctx, "tcp", address, topic, partition)
}
