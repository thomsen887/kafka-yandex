package main

import (
    "practic3/blocker"
    "practic3/emitter"
    "practic3/filter"



    "github.com/lovoo/goka"
)

var brokers = []string{"10.19.21.58:9094"}

var (
    MessagesTopic         goka.Stream = "pr3_messages"
    FilteredMessagesTopic goka.Stream = "pr3_filtered_messages"
    BlockedUsersTopic     goka.Stream = "pr3_blocked_users"
    BannedWords           goka.Stream = "pr3_banned_words"
    ForFilterTopic        goka.Stream = "pr3_for_filter"


)

func main() {

    go blocker.RunBlockerProcessor(brokers, BlockedUsersTopic)
    go filter.RunBanAggregatorProcessor(brokers, BannedWords)


    
    go emitter.RunEmitter(brokers, MessagesTopic)
    go blocker.RunUserProcessor(brokers, MessagesTopic, ForFilterTopic)
    go filter.RunFilterProcessor(brokers, ForFilterTopic, FilteredMessagesTopic)


    // Блокируем main
    select {}
}


