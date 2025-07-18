package blocker

import (
    "context"
    "encoding/json"
    "log"

    "github.com/lovoo/goka"
)

// --- Общие структуры и кодеки ---

type BlockCommand struct {
    TargetUser string
    Unblock    bool
}

type BlockList struct {
    Blocked map[string]bool
}

type Message struct {
    SenderID    string
    RecipientID string
    Content     string
}

type BlockCommandCodec struct{}

func (c *BlockCommandCodec) Encode(value any) ([]byte, error) {
    return json.Marshal(value)
}
func (c *BlockCommandCodec) Decode(data []byte) (any, error) {
    var cmd BlockCommand
    err := json.Unmarshal(data, &cmd)
    return &cmd, err
}

type BlockListCodec struct{}

func (c *BlockListCodec) Encode(value any) ([]byte, error) {
    return json.Marshal(value)
}
func (c *BlockListCodec) Decode(data []byte) (any, error) {
    var bl BlockList
    err := json.Unmarshal(data, &bl)
    return &bl, err
}

type MessageCodec struct{}

func (c *MessageCodec) Encode(value any) ([]byte, error) {
    return json.Marshal(value)
}
func (c *MessageCodec) Decode(data []byte) (any, error) {
    var msg Message
    err := json.Unmarshal(data, &msg)
    return &msg, err
}

// --- Обработка команд блокировки ---

func processBlockCommand(ctx goka.Context, msg any) {
    cmd := msg.(*BlockCommand)

    var state *BlockList
    if v := ctx.Value(); v != nil {
        state = v.(*BlockList)
    } else {
        state = &BlockList{Blocked: make(map[string]bool)}
    }

    if cmd.Unblock {
        delete(state.Blocked, cmd.TargetUser)
        log.Printf("[blocker] %s РАЗБЛОКИРОВАЛ %s", ctx.Key(), cmd.TargetUser)
    } else {
        state.Blocked[cmd.TargetUser] = true
        log.Printf("[blocker] %s ЗАБЛОКИРОВАЛ %s", ctx.Key(), cmd.TargetUser)
    }

    ctx.SetValue(state)
}

func RunBlockerProcessor(brokers []string, topic goka.Stream) {
    group := goka.Group("pr3_blocked_users")

    g := goka.DefineGroup(group,
        goka.Input(topic, new(BlockCommandCodec), processBlockCommand),
        goka.Persist(new(BlockListCodec)),
    )

    p, err := goka.NewProcessor(brokers, g)
    if err != nil {
        log.Fatal(err)
    }
    if err := p.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}

// --- Обработка сообщений между пользователями ---

func RunUserProcessor(brokers []string, input goka.Stream, output goka.Stream) {
    group := goka.Group("pr3_user_processor")

    g := goka.DefineGroup(group,
        goka.Input(input, new(MessageCodec), func(ctx goka.Context, msg any) {
            m := msg.(*Message)

            val := ctx.Join(goka.GroupTable("pr3_blocked_users"))
            if val != nil {
                bl := val.(*BlockList)
                if bl.Blocked != nil && bl.Blocked[m.SenderID] {
                    log.Printf("[user] ❌ Блокируем сообщение от %s к %s: %s", m.SenderID, m.RecipientID, m.Content)
                    return
                }
            }

            log.Printf("[user] ✅ Пропускаем сообщение от %s к %s: %s", m.SenderID, m.RecipientID, m.Content)
            ctx.Emit(output, ctx.Key(), m)
        }),
        goka.Join(goka.GroupTable("pr3_blocked_users"), new(BlockListCodec)),
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
