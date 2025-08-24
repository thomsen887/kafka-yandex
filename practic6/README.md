# Практическая работа 6 — SSL/TLS + SASL/PLAIN и ACL в Apache Kafka (KRaft)

**Цель:** развернуть кластер Kafka из трёх брокеров в режиме KRaft, включить TLS + SASL/PLAIN для всех каналов, настроить ACL, создать топики и проверить права с помощью Go-демо.

---

## Содержание

- [Схема окружения](#схема-окружения)
- [Структура репозитория](#структура-репозитория)
- [Подготовка сертификатов](#подготовка-сертификатов)
- [Запуск кластера](#запуск-кластера)
- [Проверка конфигурации](#проверка-конфигурации)
- [Создание топиков](#создание-топиков)
- [ACL: права доступа](#acl-права-доступа)
- [Go-демо: продюсер/консюмер](#go-демо-продюсерконсюмер)
- [Ожидаемый результат](#ожидаемый-результат)
- [Траблшутинг](#траблшутинг)
- [Очистка](#очистка)
- [Примечания](#примечания)

---

## Схема окружения

**Kafka (Bitnami, 3 брокера, KRaft)**

- Внутренний listener: `INTERNAL://:9092` (SASL_SSL)  
- Контроллер: `CONTROLLER://:9093` (SASL_SSL)  
- Внешний listener: `EXTERNAL://:9094/9095/9096` (SASL_SSL)

**Пользователи и пароли** (управляются переменными окружения Bitnami):

- Controller: `${KAFKA_CONTROLLER_USER}:${KAFKA_CONTROLLER_PASSWORD}`
- Inter-broker: `${KAFKA_INTER_BROKER_USER}:${KAFKA_INTER_BROKER_PASSWORD}`
- Клиенты: `admin/admin-secret`, `producer/producer-secret`, `consumer/consumer-secret`

**Цели по ACL**

- `topic-1`: `producer` пишет, `consumer` читает
- `topic-2`: `producer` пишет; `consumer` **не может** читать

---

## Структура репозитория

```
practic6/
├─ certs/
│  ├─ ca/                 # CA (ключ/сертификат)
│  ├─ kafka-0/            # keystore для kafka-0
│  ├─ kafka-1/            # keystore для kafka-1
│  ├─ kafka-2/            # keystore для kafka-2
│  └─ truststore.jks      # общий truststore
├─ docker-compose.yml
└─ go-acl-demo/
   ├─ go.mod
   ├─ go.sum
   └─ main.go
```

---

## Подготовка сертификатов

Сертификаты уже сгенерированы и лежат в `certs/`. Если нужно пересоздать, общий алгоритм:

1. Сгенерировать CA (корневой ключ/сертификат).  
2. Для каждого брокера: создать `keystore.jks` с CN/SAN на его hostname (`kafka-0`, `kafka-1`, `kafka-2`).  
3. Создать общий `truststore.jks` и добавить туда CA.

> В этой работе hostname-валидация на клиентской стороне отключена (для простоты проверки). Для продакшена добавьте корректные SAN и включите проверку имён.

---

## Запуск кластера

```bash
docker compose up -d
docker compose ps
```

**Открытые порты:**

- `9094` → `kafka-0` (EXTERNAL)
- `9095` → `kafka-1` (EXTERNAL)
- `9096` → `kafka-2` (EXTERNAL)

Опционально: `8080` для kafka-ui (если добавите сервис).

---

## Проверка конфигурации

Убедитесь, что брокеры действительно поднялись с TLS+SASL и нужными listener’ами:

```bash
docker compose exec kafka-0 bash -lc 'grep -E "^(listeners=|listener\.security\.protocol\.map=|controller\.listener\.names=|inter\.broker\.listener\.name=|sasl\.(enabled\.mechanisms|mechanism\.(controller|inter\.broker)\.protocol)=|listener\.name\.(controller|internal|external)\.(sasl\.enabled\.mechanisms|plain\.sasl\.jaas\.config)=)" /opt/bitnami/kafka/config/server.properties'
```

**Ожидаемо увидите:**

```
listener.security.protocol.map=CONTROLLER:SASL_SSL,INTERNAL:SASL_SSL,EXTERNAL:SASL_SSL
sasl.mechanism.controller.protocol=PLAIN
sasl.mechanism.inter.broker.protocol=PLAIN
sasl.enabled.mechanisms=PLAIN
```

Пароли и JAAS внутри `server.properties` проставляются автоматически на основе переменных:

- `KAFKA_CONTROLLER_USER` / `KAFKA_CONTROLLER_PASSWORD`
- `KAFKA_INTER_BROKER_USER` / `KAFKA_INTER_BROKER_PASSWORD`
- `KAFKA_CLIENT_USERS` / `KAFKA_CLIENT_PASSWORDS`

---

## Создание топиков

Создаём топики как администратор через **внутренний** listener (SASL_SSL):

```bash
docker compose exec kafka-0 bash -lc '
cat >/tmp/admin.properties <<EOF
security.protocol=SASL_SSL
sasl.mechanism=PLAIN
sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username="admin" password="admin-secret";
ssl.truststore.location=/bitnami/kafka/config/certs/kafka.truststore.jks
ssl.truststore.password=changeit
ssl.endpoint.identification.algorithm=
EOF

/opt/bitnami/kafka/bin/kafka-topics.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties --create --topic topic-1 --partitions 3 --replication-factor 3 &&
/opt/bitnami/kafka/bin/kafka-topics.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties --create --topic topic-2 --partitions 3 --replication-factor 3 &&
/opt/bitnami/kafka/bin/kafka-topics.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties --list
'
```

---

## ACL: права доступа

### `topic-1` — producer пишет, consumer читает

```bash
docker compose exec kafka-0 bash -lc '
cat >/tmp/admin.properties <<EOF
security.protocol=SASL_SSL
sasl.mechanism=PLAIN
sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username="admin" password="admin-secret";
ssl.truststore.location=/bitnami/kafka/config/certs/kafka.truststore.jks
ssl.truststore.password=changeit
ssl.endpoint.identification.algorithm=
EOF

# Producer (права на запись + описать)
/opt/bitnami/kafka/bin/kafka-acls.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties   --add --allow-principal User:producer --topic topic-1 --operation WRITE --operation DESCRIBE

# Consumer (права на чтение топика + join к группе)
/opt/bitnami/kafka/bin/kafka-acls.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties   --add --allow-principal User:consumer --topic topic-1 --operation READ --operation DESCRIBE

/opt/bitnami/kafka/bin/kafka-acls.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties   --add --allow-principal User:consumer --group group-1 --operation READ
'
```

### `topic-2` — producer пишет, consumer **читать не может**

Достаточно **не давать** `consumer` никаких READ-прав. Для наглядности можно ещё явно запретить:

```bash
docker compose exec kafka-0 bash -lc '
# Producer может писать в topic-2
/opt/bitnami/kafka/bin/kafka-acls.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties   --add --allow-principal User:producer --topic topic-2 --operation WRITE --operation DESCRIBE

# Явный запрет (опционально; Deny имеет приоритет)
/opt/bitnami/kafka/bin/kafka-acls.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties   --add --deny-principal User:consumer --topic topic-2 --operation READ
'
```

Проверить текущие ACL:

```bash
docker compose exec kafka-0 bash -lc '/opt/bitnami/kafka/bin/kafka-acls.sh --bootstrap-server kafka-0:9092 --command-config /tmp/admin.properties --list'
```

---

## Go-демо: продюсер/консюмер

Код расположен в `go-acl-demo/` (используется `github.com/segmentio/kafka-go`).

**Зависимости**

- Go `1.24.x`
- Модуль `kafka-go` версии `v0.4.45` (уже зафиксирован в `go.mod`)

**Переменные окружения**

```bash
export BROKERS="10.19.21.58:9094,10.19.21.58:9095,10.19.21.58:9096"

# producer (пишет в topic-1 и topic-2)
export PRODUCER_USERNAME=producer
export PRODUCER_PASSWORD=producer-secret

# consumer (читает topic-1, но НЕ читает topic-2)
export CONSUMER_USERNAME=consumer
export CONSUMER_PASSWORD=consumer-secret
export CONSUMER_GROUP=group-1

# для упрощения демонстрации отключаем проверку имени хоста в TLS
export TLS_INSECURE=true
```

**Запуск**

```bash
cd go-acl-demo
go run .
```

---

## Ожидаемый результат

Пример вывода:

```
Producing messages...
✅ Produced OK

Consuming from topic-1 …
  [topic-1] key=k2 value="second message to topic-1"
  [topic-1] key=k1 value="hello from producer to topic-1 @ 2025-08-24T21:16:19+03:00"
✅ Read from topic-1 OK

Consuming from topic-2 … (ожидаем ошибку авторизации)
❌ As expected, failed to read topic-2: context deadline exceeded
```

Это подтверждает:

- `producer` имеет право писать в оба топика,  
- `consumer` может читать `topic-1`,  
- `consumer` **не может** читать `topic-2` (ACL работает).

---

## Траблшутинг

### `Authentication failed: Invalid username or password`

Проверь, что пользователь есть в переменных окружения компоуз-файла:  
`KAFKA_CLIENT_USERS="admin,producer,consumer"` и соответствующие пароли в `KAFKA_CLIENT_PASSWORDS`.  
Перезапусти контейнеры.

### `UnsupportedCallbackException` / `Failed to create SaslClient` при старте брокеров

В KRaft всегда укажи:

- `KAFKA_CFG_SASL_MECHANISM_CONTROLLER_PROTOCOL=PLAIN`
- `KAFKA_CFG_SASL_MECHANISM_INTER_BROKER_PROTOCOL=PLAIN`

Не задавай вручную JAAS-строки через `KAFKA_CFG_*JAAS_CONFIG`, доверь это Bitnami (`*_USER/PASSWORD`).

### Проблемы с TLS именами

Для быстрого старта мы отключили hostname-валидацию (в `admin.properties` `ssl.endpoint.identification.algorithm=` и в Go `TLS_INSECURE=true`).  
Для продакшена добавьте корректные SAN в сертификаты и включите проверку.

### Проверка активной конфигурации брокера

```bash
docker compose exec kafka-0 bash -lc 'grep -E "^(listeners=|listener\.security\.protocol\.map=|sasl\.(enabled\.mechanisms|mechanism\.(controller|inter\.broker)\.protocol)=|inter\.broker\.listener\.name=|listener\.name\.(controller|internal|external)\.(sasl\.enabled\.mechanisms|plain\.sasl\.jaas\.config)=)" /opt/bitnami/kafka/config/server.properties'
```

---

## Очистка

```bash
docker compose down -v
# удалит контейнеры и тома kafka_*_data
```

---

## Примечания

- Контроллерный канал (KRaft) в текущих версиях поддерживает только SASL/PLAIN, поэтому для него мы используем PLAIN + TLS.
- Пароли и JAAS в `server.properties` Bitnami формирует сам из переменных окружения — это главный анти-футган в данной конфигурации.
