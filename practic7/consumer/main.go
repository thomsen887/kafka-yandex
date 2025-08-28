package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

func main() {
	// ВШИТЫЕ значения
	const brokers = "rc1a-659o7je94ifi6m54.mdb.yandexcloud.net:9091,rc1a-9mrrj4cfe4oknd8n.mdb.yandexcloud.net:9091,rc1a-gk0f7bbj1363mc1s.mdb.yandexcloud.net:9091"
	const username = "user1_kafka"
	const password = "Password@11"
	const topic = "topic-1"
	const caPath = "/usr/local/share/ca-certificates/Yandex/YandexInternalRootCA.crt"

	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	splitBrokers := strings.Split(brokers, ",")

	conf := sarama.NewConfig()
	conf.ClientID = "sasl_scram_client_consumer"
	conf.Version = sarama.V0_10_0_0
	conf.Consumer.Return.Errors = true
	conf.Metadata.Full = true

	// Таймауты
	conf.Net.DialTimeout = 10 * time.Second
	conf.Net.ReadTimeout = 10 * time.Second
	conf.Net.WriteTimeout = 10 * time.Second

	// SASL SCRAM
	conf.Net.SASL.Enable = true
	conf.Net.SASL.Handshake = true
	conf.Net.SASL.User = username
	conf.Net.SASL.Password = password
	conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
		return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
	}
	conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	conf.Net.SASL.Version = sarama.SASLHandshakeV0

	certs := x509.NewCertPool()
	pemData, err := os.ReadFile("/usr/local/share/ca-certificates/Yandex/YandexInternalRootCA.crt")
	if err != nil { log.Fatalf("CA read err: %v", err) }
	if !certs.AppendCertsFromPEM(pemData) { log.Fatal("Append CA failed") }

	conf.Net.TLS.Enable = true
	conf.Net.TLS.Config = &tls.Config{
    	MinVersion: tls.VersionTLS12,
    	RootCAs:    certs,
	}

	c, err := sarama.NewConsumer(splitBrokers, conf)
	if err != nil {
		log.Fatalf("Couldn't create consumer: %v", err)
	}
	defer c.Close()

	pc, err := c.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		log.Fatalf("ConsumePartition err: %v", err)
	}
	defer pc.Close()

	fmt.Println("Consuming... (Ctrl+C to stop)")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	count := 0
loop:
	for {
		select {
		case err := <-pc.Errors():
			log.Printf("ERR: %v", err)
		case m := <-pc.Messages():
			count++
			fmt.Printf("%s | partition=%d offset=%d key=%q value=%q\n",
				time.Now().Format(time.RFC3339), m.Partition, m.Offset, string(m.Key), string(m.Value))
		case <-sig:
			break loop
		}
	}
	fmt.Printf("Processed %d messages\n", count)
}
