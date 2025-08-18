package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/segmentio/kafka-go"
)

func main() {
        ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigCh
        fmt.Println("\nStopping consumer…")
        cancel()
    }()

    brokers := []string{"10.19.21.58:9094"}


    topic := "mytopic" 


    groupID := "go-consumer-group"


    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:  brokers,
        GroupID:  groupID,
        Topic:    topic,
        MinBytes: 1,       //  fetch messages as soon as they arrive
        MaxBytes: 10e6,    //  10 MB
    })
    defer func() {
        if err := reader.Close(); err != nil {
            log.Printf("failed to close reader: %v", err)
        }
    }()

    fmt.Printf("Listening for messages on topic %s…\n", topic)

    for {
        m, err := reader.ReadMessage(ctx)
        if err != nil {
            // If the context has been cancelled (e.g. via Ctrl+C), exit the loop.
            if ctx.Err() != nil {
                break
            }
            log.Printf("error reading message: %v", err)
            continue
        }
        // Print the message value.  Keys are typically empty for Debezium events.
        fmt.Printf("offset=%d partition=%d topic=%s\nvalue: %s\n\n", m.Offset, m.Partition, m.Topic, string(m.Value))
    }

    fmt.Println("Consumer stopped.")
}