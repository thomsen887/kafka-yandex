# Практическая работа: Kafka в Yandex Cloud (Задание 1)

## Содержание
- [Архитектура и ресурсы](#архитектура-и-ресурсы)
- [Создание топика и параметры хранения](#создание-топика-и-параметры-хранения)
- [Schema Registry: регистрация и проверка](#schema-registry-регистрация-и-проверка)
- [Клиентские настройки (SASL_SSL SCRAM)](#клиентские-настройки-sasl_ssl-scram)
- [Проверка работы кластера](#проверка-работы-кластера)
- [Код продюсера и консьюмера](#код-продюсера-и-консьюмера)
- [Файлы схем](#файлы-схем)
- [Артефакты для сдачи](#артефакты-для-сдачи)

---

## Архитектура и ресурсы

### Параметры кластера
- **Имя**: `kafka138`
- **Идентификатор**: `c9qo95jpq5c5bb441jd2`
- **Дата создания**: 26.08.2025 11:34
- **Окружение**: PRODUCTION
- **Версия Kafka**: 3.9
- **Реестр схем данных**: включён ✅
- **Kafka REST API**: включён ✅
- **Кластер отказоустойчивый**: нет
- **Kafka UI**: доступен через консоль

### Ресурсы брокеров
- **Количество брокеров**: 3
- **Класс хоста**: `b3-c1-m4` (2 vCPU, 50% vCPU rate, 4 ГБ RAM)
- **Хранилище**: 32 ГБ SSD (network-ssd)

---

## Создание топика и параметры хранения

Создан топик **`topic-1`**:
- **Партиций**: 3
- **Фактор репликации**: 3
- **Политика очистки**: `cleanup.policy=delete`
- **retention.ms**: `86400000` (1 день)
- **segment.bytes**: `268435456` (256 MiB)

### Фактический вывод
```text
Topic: topic-1	TopicId: TBl1pe7mRXG6FoA4rxf1tg	PartitionCount: 3	ReplicationFactor: 3	Configs: min.insync.replicas=2,cleanup.policy=delete,segment.bytes=268435456,retention.ms=86400000
	Topic: topic-1	Partition: 0	Leader: 3	Replicas: 3,1,2	Isr: 3,1,2	Elr: 	LastKnownElr:
	Topic: topic-1	Partition: 1	Leader: 1	Replicas: 1,2,3	Isr: 3,1,2	Elr: 	LastKnownElr:
	Topic: topic-1	Partition: 2	Leader: 2	Replicas: 2,3,1	Isr: 3,1,2	Elr: 	LastKnownElr:
```

Команда:
```bash
kafka-topics.sh   --bootstrap-server rc1a-659o7je94ifi6m54.mdb.yandexcloud.net:9091   --command-config client.properties   --describe --topic topic-1
```

---

## Schema Registry: регистрация и проверка

В Managed Kafka от Yandex Cloud Schema Registry доступен по HTTPS на 443 (Karapace, совместим с Confluent API). Зарегистрированы схемы для сабджектов:

- `topic-1-key` (Avro)
- `topic-1-value` (Avro)

### Проверки

Переменные окружения:
```bash
export BROKER_FQDN=rc1a-659o7je94ifi6m54.mdb.yandexcloud.net
export USER=user1_kafka
export PASS='Password@11'
export CA=/usr/local/share/ca-certificates/Yandex/YandexInternalRootCA.crt
```

Список сабджектов:
```bash
curl --cacert "$CA" --user "$USER:$PASS"   -H 'Accept: application/vnd.schemaregistry.v1+json'   "https://$BROKER_FQDN:443/subjects"
```
Фактический вывод:
```json
["topic-1-key","topic-1-value"]
```

Версии `topic-1-value`:
```bash
curl --cacert "$CA" --user "$USER:$PASS"   -H 'Accept: application/vnd.schemaregistry.v1+json'   "https://$BROKER_FQDN:443/subjects/topic-1-value/versions"
```
Фактический вывод:
```json
[1]
```

---

## Клиентские настройки (SASL_SSL SCRAM)

**client.properties** (для Java-утилит Kafka):
```properties
security.protocol=SASL_SSL
sasl.mechanism=SCRAM-SHA-512
# Подставьте свои креды:
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required  username="user1_kafka" password="Password@11";

# Truststore с корневым сертификатом Яндекса
ssl.truststore.location=./kafka-truststore.jks
ssl.truststore.password=changeit
ssl.endpoint.identification.algorithm=https
```

Создание truststore из CA:
```bash
keytool -importcert   -alias yandex-ca   -file /usr/local/share/ca-certificates/Yandex/YandexInternalRootCA.crt   -keystore kafka-truststore.jks   -storepass changeit -noprompt
```

Переменные окружения, используемые при работе с Registry:
```bash
export BROKER_FQDN=rc1a-659o7je94ifi6m54.mdb.yandexcloud.net
export USER=user1_kafka
export PASS='Password@11'
export CA=/usr/local/share/ca-certificates/Yandex/YandexInternalRootCA.crt
```

---

## Проверка работы кластера

### Продюсер (Go)
Запуск:
```bash
./producer/producer
```
Фактический лог:
```text
[sarama] 2025/08/28 10:08:36 Initializing new client
...
2025/08/28 10:08:37 Produced -> partition=0 offset=5
...
2025/08/28 10:08:37 Produced -> partition=1 offset=12
2025/08/28 10:08:37 Produced -> partition=1 offset=13
...
2025/08/28 10:08:37 Produced -> partition=2 offset=8
2025/08/28 10:08:37 Produced -> partition=2 offset=9
[sarama] 2025/08/28 10:08:37 Producer shutting down.
```

### Консьюмер (Go)
Запуск (в другом терминале):
```bash
./consumer/consumer
```
Фактический лог:
```text
[sarama] 2025/08/28 10:08:23 Initializing new client
...
Consuming... (Ctrl+C to stop)
2025-08-28T10:08:23+03:00 | partition=0 offset=2 key="" value="test message 2 @ 2025-08-28T10:07:54+03:00"
2025-08-28T10:08:23+03:00 | partition=0 offset=3 key="" value="test message 5 @ 2025-08-28T10:07:55+03:00"
2025-08-28T10:08:26+03:00 | partition=0 offset=4 key="" value="test message 2 @ 2025-08-28T10:08:26+03:00"
2025-08-28T10:08:37+03:00 | partition=0 offset=5 key="" value="test message 1 @ 2025-08-28T10:08:37+03:00"
```

---

## Код продюсера и консьюмера

Файлы из репозитория:
- `main_producer.go` — отправляет текстовые сообщения в `topic-1`.
- `main_consumer.go` — читает сообщения из `topic-1`.
- `scram.go` — реализация SCRAM-SHA-512 для Sarama (если используется кастомная).

---

## Файлы схем

### schema-key.json
```json
{
  "type": "record",
  "name": "key_v1",
  "fields": [
    { "name": "id", "type": "int" }
  ]
}
```

### schema-value.json
```json
{
  "type": "record",
  "name": "value_v1",
  "fields": [
    { "name": "name", "type": "string" },
    { "name": "city", "type": "string" },
    { "name": "age", "type": "int" }
  ]
}
```





# Задание 2. Интеграция Kafka с внешними системами (Apache NiFi)

## Выбор инструмента
Выбран **Apache NiFi** как наиболее быстрый и наглядный инструмент интеграции.

## Запуск NiFi (Docker)
`docker-compose.yml`:
```yaml
services:
  nifi:
    image: apache/nifi:1.26.0
    container_name: nifi
    ports:
      - "8080:8080"
    environment:
      - NIFI_WEB_HTTP_PORT=8080
    volumes:
      - /home/etomsen/kafka-yandex/practic7/nifi/certs:/opt/nifi/nifi-current/certs:ro
    restart: unless-stopped
```
Truststore (`kafka-truststore.jks`) размещён на хосте в `./nifi/certs/` и виден в контейнере по пути `/opt/nifi/nifi-current/certs/kafka-truststore.jks`.

Команды:
```bash
docker compose up -d
docker ps
docker exec -it nifi bash -lc 'ls -l /opt/nifi/nifi-current/certs'
```

## Настройка контроллер-сервиса SSL
**StandardSSLContextService** (Controller Services):
- Truststore Filename: `/opt/nifi/nifi-current/certs/kafka-truststore.jks`
- Truststore Type: `JKS`
- Truststore Password: `changeit`
- TLS Protocol: `TLSv1.2`
- Keystore-поля: пустые
Статус сервиса: **Enabled**.

## Поток NiFi
Пайплайн: **GenerateFlowFile** → **PublishKafka_2_6**

**GenerateFlowFile**
- Custom Text: `hello from nifi`
- Scheduling → Run Schedule: `1 sec`

**PublishKafka_2_6 → Properties**
- Kafka Brokers: `rc1a-659o7je94ifi6m54.mdb.yandexcloud.net:9091,rc1a-9mrrj4cfe4oknd8n.mdb.yandexcloud.net:9091,rc1a-gk0f7bbj1363mc1s.mdb.yandexcloud.net:9091`
- Topic Name: `topic-1`
- Delivery Guarantee: `1`
- Security Protocol: `SASL_SSL`
- SASL Mechanism: `SCRAM-SHA-512`
- Username: `user1_kafka`
- Password: `Password@11`
- SSL Context Service: `StandardSSLContextService`

**PublishKafka_2_6 → Relationships**
- `success`: Auto-terminate
- `failure`: Auto-terminate

## Проверка доставки
Консьюмер (Go) — фактический вывод (фрагмент):
```text
Consuming... (Ctrl+C to stop)
2025-08-28T14:42:11+03:00 | partition=0 offset=27 key="" value="hello from nifi"
2025-08-28T14:42:11+03:00 | partition=0 offset=28 key="" value="hello from nifi"
2025-08-28T14:42:11+03:00 | partition=0 offset=29 key="" value="hello from nifi"
...
2025-08-28T14:42:12+03:00 | partition=0 offset=40 key="" value="hello from nifi"
```

Альтернативная проверка (CLI):
```bash
kafka-console-consumer.sh   --bootstrap-server rc1a-659o7je94ifi6m54.mdb.yandexcloud.net:9091   --consumer.config /home/etomsen/kafka-yandex/practic7/client.properties   --topic topic-1 --from-beginning --timeout-ms 10000
```

## Что приложено по заданию 2
- **Скриншот UI NiFi** с запущенными процессорами (GenerateFlowFile и PublishKafka_2_6).
- **Скриншот/вывод `docker ps`** для подтверждения запуска сервиса.
- **Конфигурация запуска**: `docker-compose.yml` (см. выше).
- **Логи успешной передачи**: вывод Go-консьюмера (см. выше).
- **Параметры процессоров**: описаны полями 

*Все скриншоты в папке Screenshots*