package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func makeDialer(user, pass string, insecure bool) *kafka.Dialer {
	mech := plain.Mechanism{
		Username: user,
		Password: pass,
	}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: insecure, // true — если брокеры по IP, а сертификат на hostname
		MinVersion:         tls.VersionTLS12,
	}
	return &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsCfg,
		SASLMechanism: mech,
	}
}

func newWriter(brokers []string, topic string, d *kafka.Dialer) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:      brokers,
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: int(kafka.RequireAll), // ждём подтверждения всех реплик
		Async:        false,
		Dialer:       d,
		// можно добавить Logger/ErrorLogger при желании
	})
}

func newReader(brokers []string, topic, group string, d *kafka.Dialer) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     group,
		Topic:       topic,
		MinBytes:    1,          // 1B
		MaxBytes:    10e6,       // 10MB
		StartOffset: kafka.FirstOffset,
		MaxWait:     500 * time.Millisecond,
		Dialer:      d,
	})
}

func must(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

func main() {
	// --- env ---
	brokers := strings.Split(getenv("BROKERS", "127.0.0.1:9092"), ",")
	insecure := strings.EqualFold(getenv("TLS_INSECURE", "true"), "true")

	producerUser := getenv("PRODUCER_USERNAME", "producer")
	producerPass := getenv("PRODUCER_PASSWORD", "producer-secret")

	consumerUser := getenv("CONSUMER_USERNAME", "consumer")
	consumerPass := getenv("CONSUMER_PASSWORD", "consumer-secret")
	consumerGroup := getenv("CONSUMER_GROUP", "group-1")

	topic1 := getenv("TOPIC1", "topic-1")
	topic2 := getenv("TOPIC2", "topic-2")

	fmt.Printf("brokers=%v, insecureTLS=%v\n", brokers, insecure)
	fmt.Printf("producer=%s  consumer=%s  group=%s\n", producerUser, consumerUser, consumerGroup)
	fmt.Printf("topics: %s, %s\n\n", topic1, topic2)

	// --- dialers ---
	prodDialer := makeDialer(producerUser, producerPass, insecure)
	consDialer := makeDialer(consumerUser, consumerPass, insecure)

	// --- writers ---
	w1 := newWriter(brokers, topic1, prodDialer)
	defer w1.Close()
	w2 := newWriter(brokers, topic2, prodDialer)
	defer w2.Close()

	// --- send some messages ---
	now := time.Now().Format(time.RFC3339)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("Producing messages...")
	err := w1.WriteMessages(ctx,
		kafka.Message{Key: []byte("k1"), Value: []byte("hello from producer to " + topic1 + " @ " + now)},
		kafka.Message{Key: []byte("k2"), Value: []byte("second message to " + topic1)},
	)
	must(err, "produce to "+topic1)

	err = w2.WriteMessages(ctx,
		kafka.Message{Key: []byte("k1"), Value: []byte("hello from producer to " + topic2 + " @ " + now)},
	)
	must(err, "produce to "+topic2)

	fmt.Println("✅ Produced OK\n")

	// --- readers ---
	r1 := newReader(brokers, topic1, consumerGroup, consDialer)
	defer r1.Close()

	r2 := newReader(brokers, topic2, consumerGroup, consDialer)
	defer r2.Close()

	// consume from topic-1 (должно сработать)
	fmt.Println("Consuming from", topic1, "…")
	readN := 0
	readCtx, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRead()
	for readN < 2 {
		m, err := r1.FetchMessage(readCtx)
		if err != nil {
			must(err, "read from "+topic1)
		}
		fmt.Printf("  [%s] key=%s value=%q\n", topic1, string(m.Key), string(m.Value))
		_ = r1.CommitMessages(context.Background(), m)
		readN++
	}
	fmt.Println("✅ Read from", topic1, "OK\n")

	// try to consume from topic-2 (ожидаем отказ по ACL)
	fmt.Println("Consuming from", topic2, "… (ожидаем ошибку авторизации)")
	tryCtx, cancelTry := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancelTry()
	if m, err := r2.FetchMessage(tryCtx); err != nil {
		fmt.Printf("❌ As expected, failed to read %s: %v\n", topic2, err)
	} else {
		// если ACL всё же разрешает чтение, покажем сообщение (на случай проверки)
		fmt.Printf("⚠️  Unexpectedly read from %s: key=%s value=%q\n", topic2, string(m.Key), string(m.Value))
		_ = r2.CommitMessages(context.Background(), m)
	}
}
