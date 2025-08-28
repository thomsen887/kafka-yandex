package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strings"
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
	conf.ClientID = "sasl_scram_client_producer"
	conf.Version = sarama.V0_10_0_0

	// Таймауты и метаданные
	conf.Net.DialTimeout = 10 * time.Second
	conf.Net.ReadTimeout = 10 * time.Second
	conf.Net.WriteTimeout = 10 * time.Second
	conf.Metadata.Retry.Max = 3
	conf.Metadata.Retry.Backoff = 500 * time.Millisecond
	conf.Metadata.RefreshFrequency = 30 * time.Second

	// Producer
	conf.Producer.RequiredAcks = sarama.WaitForAll
	conf.Producer.Return.Successes = true
	conf.Producer.Retry.Max = 3

	// SASL SCRAM-SHA-512
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

	producer, err := sarama.NewSyncProducer(splitBrokers, conf)
	if err != nil {
		log.Fatalf("Couldn't create producer: %v", err)
	}
	defer producer.Close()

	for i := 1; i <= 5; i++ {
		msg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder(fmt.Sprintf("test message %d @ %s", i, time.Now().Format(time.RFC3339))),
		}
		p, o, err := producer.SendMessage(msg)
		if err != nil {
			log.Printf("Error publish: %v", err)
			continue
		}
		log.Printf("Produced -> partition=%d offset=%d\n", p, o)
	}
}
