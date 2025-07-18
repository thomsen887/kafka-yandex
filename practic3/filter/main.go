package filter

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"strings"

	"github.com/lovoo/goka"
)

// --- Типы сообщений ---

type Message struct {
	SenderID    string
	RecipientID string
	Content     string
}

type MessageCodec struct{}

func (c *MessageCodec) Encode(value any) ([]byte, error) { return json.Marshal(value) }
func (c *MessageCodec) Decode(data []byte) (any, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return &m, err
}

// --- Обновления слов ---

type WordUpdate struct {
	Word   string
	Active bool
}

type WordUpdateCodec struct{}

func (c *WordUpdateCodec) Encode(value any) ([]byte, error) { return json.Marshal(value) }
func (c *WordUpdateCodec) Decode(data []byte) (any, error) {
	var wu WordUpdate
	err := json.Unmarshal(data, &wu)
	return &wu, err
}

// --- Таблица банов ---

type BannedWords struct {
	Words map[string]bool
}

type BannedWordsCodec struct{}

func (c *BannedWordsCodec) Encode(value any) ([]byte, error) { return json.Marshal(value) }
func (c *BannedWordsCodec) Decode(data []byte) (any, error) {
	var bw BannedWords
	err := json.Unmarshal(data, &bw)
	return &bw, err
}

// --- Агрегатор запрещённых слов ---
func RunBanAggregatorProcessor(brokers []string, topic goka.Stream) {
	group := goka.Group("pr3_banned_words") // 👈 имя группы и таблицы

	g := goka.DefineGroup(group,
		goka.Input(topic, new(WordUpdateCodec), func(ctx goka.Context, msg any) {
			
            update := msg.(*WordUpdate)

			var state *BannedWords
			if val := ctx.Value(); val != nil {
				state = val.(*BannedWords)
			} else {
				state = &BannedWords{Words: make(map[string]bool)}
			}

			state.Words[strings.ToLower(update.Word)] = update.Active 
			ctx.SetValue(state)

			if update.Active {
				log.Printf("[banagg] ✅ '%s' добавлено в бан", update.Word)
			} else {
				log.Printf("[banagg] ❌ '%s' удалено из бана", update.Word)
			}
		}),
		goka.Persist(new(BannedWordsCodec)), 
	)

	p, err := goka.NewProcessor(brokers, g)
	if err != nil {
		log.Fatal(err)
	}
	if err := p.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
    
}


func RunFilterProcessor(brokers []string, input goka.Stream, output goka.Stream) {
	group := goka.Group("pr3_banwriter")

	log.Println("[filter] 🔄 Запускаем фильтр с использованием Lookup")

	g := goka.DefineGroup(group,
		goka.Input(input, new(MessageCodec), func(ctx goka.Context, msg any) {
			m := msg.(*Message)
			log.Printf("[filter] 📥 Получено сообщение от %s: %s", m.SenderID, m.Content)

			val := ctx.Lookup(goka.GroupTable("pr3_banned_words"), "system")
			if val == nil {
				log.Println("[filter] ⚠️ Нет данных в таблице — пропускаем")
				ctx.Emit(output, m.RecipientID, m)
				return
			}

			bw := val.(*BannedWords)
			if bw == nil || len(bw.Words) == 0 {
				log.Println("[filter] 📭 Таблица пуста — пропускаем")
				ctx.Emit(output, m.RecipientID, m)
				return
			}

	

			replaced := m.Content
			censored := false
			for word := range bw.Words {
				if bw.Words[word] {
					re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(word))
					if re.MatchString(replaced) {
						replaced = re.ReplaceAllString(replaced, "***")
						censored = true
					}
				}
			}

			if censored {
				log.Printf("[filter] 🚫 Найдены запрещённые слова — сообщение изменено: %s", replaced)
				m.Content = replaced
			} else {
				log.Println("[filter] ✅ Нет запрещённых слов — сообщение оставлено как есть")
			}

			ctx.Emit(output, m.RecipientID, m)
		}),
		goka.Lookup(goka.GroupTable("pr3_banned_words"), new(BannedWordsCodec)), 
		goka.Output(output, new(MessageCodec)), 
	)

	p, err := goka.NewProcessor(brokers, g)
	if err != nil {
		log.Fatal(err)
	}
	if err := p.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}






