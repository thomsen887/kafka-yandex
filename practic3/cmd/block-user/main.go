package main

import (
    "flag"
    "log"

    "github.com/lovoo/goka"
    "practic3/blocker"
)

var (
    brokers = []string{"10.19.21.58:9094"}
    topic   = goka.Stream("pr3_blocked_users")

    userFlag   = flag.String("user", "", "Пользователь, который блокирует")
    targetFlag = flag.String("target", "", "Целевой пользователь")
    unblock    = flag.Bool("unblock", false, "Разблокировать вместо блокировки")
)

func main() {
    flag.Parse()

    if *userFlag == "" || *targetFlag == "" {
        log.Fatal("Ошибка: необходимо указать --user и --target")
    }

    emitter, err := goka.NewEmitter(brokers, topic, new(blocker.BlockCommandCodec))
    if err != nil {
        log.Fatalf("Ошибка создания эмиттера: %v", err)
    }
    defer emitter.Finish()

    cmd := &blocker.BlockCommand{
        TargetUser: *targetFlag,
        Unblock:    *unblock,
    }

    err = emitter.EmitSync(*userFlag, cmd)
    if err != nil {
        log.Fatalf("Ошибка отправки команды блокировки: %v", err)
    }

    if *unblock {
        log.Printf("Пользователь %s разблокировал пользователя %s", *userFlag, *targetFlag)
    } else {
        log.Printf("Пользователь %s заблокировал пользователя %s", *userFlag, *targetFlag)
    }
}
