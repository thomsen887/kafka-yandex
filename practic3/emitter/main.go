package emitter

import (
    "encoding/json"
    "fmt"
    "log"
    "math/rand"
    "time"

    "github.com/lovoo/goka"
)

// Структура сообщения
type Message struct {
    SenderID    string
    RecipientID string
    Content     string
}

// Кодек для Message
type MessageCodec struct{}

func (c *MessageCodec) Encode(value any) ([]byte, error) {
    return json.Marshal(value)
}

func (c *MessageCodec) Decode(data []byte) (any, error) {
    var msg Message
    err := json.Unmarshal(data, &msg)
    return &msg, err
}

var testMessages = []string{
    "Привет, как дела?",
    "Ты просто дурак человек",
    "Это полный идиотизм",
    "Ненавижу, когда ты ведёшь себя как дебил",
    "Дурак был бы доволен таким ответом",
    "Сегодня отличная погода!",
    "Ты что, совсем тупой?",
    "Какая мерзкая идея",
    "Я думаю, ты глупец",
    "Иди на хуй!",
    "Просто тестовое сообщение для проверки фильтра",
    "This fuck",
}

// Экспортируемая функция для запуска emitter'а
func RunEmitter(brokers []string, topic goka.Stream) {
    emitter, err := goka.NewEmitter(brokers, topic, new(MessageCodec))
    if err != nil {
        log.Fatal(err)
    }
    defer emitter.Finish()

    for range time.Tick(3 * time.Second) {
        msg := &Message{
            SenderID:    fmt.Sprintf("user%d", rand.Intn(3)),
            RecipientID: fmt.Sprintf("user%d", rand.Intn(3)),
            Content:     testMessages[rand.Intn(len(testMessages))],
        }
        err := emitter.EmitSync(msg.RecipientID, msg)
        if err != nil {
            log.Fatal(err)
        }
    }
}
