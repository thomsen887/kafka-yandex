package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "log"

    "github.com/lovoo/goka"
)

type WordUpdate struct {
    Word   string
    Active bool
}

type WordUpdateCodec struct{}

func (c *WordUpdateCodec) Encode(value any) ([]byte, error) {
    return json.Marshal(value)
}
func (c *WordUpdateCodec) Decode(data []byte) (any, error) {
    var wu WordUpdate
    err := json.Unmarshal(data, &wu)
    return &wu, err
}

func main() {
    var (
        word   = flag.String("word", "", "Слово для добавления/удаления")
        delete = flag.Bool("delete", false, "Удалить слово вместо добавления")
        broker = flag.String("broker", "10.19.21.58:9094", "Kafka брокер") // ← проверь!
        topic  = flag.String("stream", "pr3_banned_words", "Kafka топик")
    )

    flag.Parse()

    if *word == "" {
        log.Fatal("Нужно указать слово через --word")
    }

    emitter, err := goka.NewEmitter([]string{*broker}, goka.Stream(*topic), new(WordUpdateCodec))
    if err != nil {
        log.Fatal(err)
    }
    defer emitter.Finish()

    event := &WordUpdate{
        Word:   *word,
        Active: !*delete,
    }

    err = emitter.EmitSync("system", event)
    if err != nil {
        log.Fatal(err)
    }

    action := "добавлено в список запрещённых"
    if *delete {
        action = "удалено из списка запрещённых"
    }
    fmt.Printf("Слово '%s' %s\n", *word, action)
}
