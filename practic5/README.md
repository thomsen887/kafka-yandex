# Практическая работа 5 — Debezium CDC + Kafka + Prometheus/Grafana

В этой работе мы настраиваем потоковую репликацию изменений из PostgreSQL в Kafka, используя коннектор Debezium, а также подключаем Prometheus и Grafana для мониторинга.  Все компоненты собираются и запускаются в Docker Compose, поэтому для воспроизведения достаточно установленного Docker Engine.

## 📦 Состав решения

В каталоге `practic5` находятся файлы и каталоги, необходимые для запуска:

| Путь | Назначение |
|---|---|
| `docker-compose.yaml` | Описывает все сервисы — Kafka, Kafka Connect, PostgreSQL, Prometheus и Grafana. |
| `postgres/` | Конфигурация PostgreSQL.  В файле `custom-config.conf` включён логический журнал (`wal_level = logical`), а `init.sql` создаёт таблицы `users` и `orders` и заполняет их тестовыми данными. |
| `kafka-connect/` | Содержит Dockerfile и настройки JMX‑экспортера для Kafka Connect (`kafka-connect.yml` и `jmx_prometheus_javaagent-0.19.0.jar`).  Папка `confluent-hub-components` включает плагин Debezium для PostgreSQL. |
| `prometheus/prometheus.yml` | Конфигурация Prometheus, которая собирает собственные метрики и метрики Kafka Connect. |
| `grafana/` | Настройки Grafana: datasource указывает на Prometheus, а дешборд `connect.json` отображает состояние коннекторов. |
| `consumer/` | Консьюмер на golang |
| `connector.json` | JSON‑конфигурация коннектора Debezium, отслеживающего только таблицы `users` и `orders` в базе `demo_db`. |


## 🚀 Быстрый старт

1. **Склонируйте репозиторий и перейдите в каталог практики.**

   ```bash
   git clone https://github.com/thomsen887/kafka‑yandex.git
   cd kafka‑yandex/practic5
   ```

2. **Запустите инфраструктуру.**
   Docker Compose создаст и запустит все контейнеры: брокер Kafka, базу PostgreSQL, Kafka Connect с плагином Debezium, а также Prometheus и Grafana для мониторинга.

   ```bash
   docker compose up -d
   ```

   После запуска можно убедиться, что сервисы в состоянии `healthy`:

   ```bash
   docker compose ps
   ```

3. **Зарегистрируйте коннектор.**
   Debezium не создаёт коннектор автоматически — его необходимо зарегистрировать через REST API Kafka Connect.  Файл `connector.json` уже содержит правильные настройки.  Выполните команду:

   ```bash
   curl -i -X POST \
     -H "Content-Type: application/json" \
     --data @connector.json \
     http://localhost:8083/connectors
   ```

   Если коннектор был успешно создан, REST API вернёт объект с его именем и конфигурацией.  Проверить статус можно так:

   ```bash
   curl -s http://localhost:8083/connectors/pg-users-orders-connector/status | jq
   ```

4. **Проверьте, что изменения из PostgreSQL поступают в Kafka.**  
   ```

   В отдельном терминале подключитесь к PostgreSQL и добавьте новые записи в таблицы `users` и `orders`.  Потребитель отобразит изменения в режиме реального времени:

   ```bash
   docker exec -it postgres psql -U postgres -d demo_db -c "INSERT INTO users (name, email) VALUES ('Charlie Chaplin', 'charlie@example.com');"
   docker exec -it postgres psql -U postgres -d demo_db -c "INSERT INTO orders (user_id, product_name, quantity) VALUES (5, 'Product F', 2);"
   ```

5. **Откройте интерфейсы мониторинга.**

   - Prometheus доступен по адресу <http://localhost:9090>.  В нём можно выполнять запросы к метрикам, например `kafka_connect_worker_connector_total_task_count`.
   - Grafana запускается на <http://localhost:3000>.  Логин/пароль по умолчанию: `admin/admin`.  Дешборд `connect.json` будет автоматически доступен и покажет количество задач, статус коннектора и другие показатели.


## 🧰 Подробности конфигурации

### PostgreSQL

Используется образ `debezium/postgres:16`, в который уже встроен логический репликационный плагин `pgoutput`.  Конфигурация `custom-config.conf` включает необходимый режим `wal_level = logical` и параметры для репликационных слотов.  При старте Postgres выполняет скрипт `init.sql`, который создаёт таблицы `users` и `orders` и заполняет их тестовыми данными.

### Kafka и Kafka Connect

Брокер Kafka развёрнут в режиме KRaft и слушает порт `9092`.  Коннектор работает в сервисе `kafka-connect`, построенном на базе образа `confluentinc/cp-kafka-connect`.  В контейнер копируются jar‑файлы JMX‑экспортера и файл правил `kafka-connect.yml`, который описывает, какие метрики экспортировать в Prometheus.  Папка `confluent-hub-components` монтируется внутрь контейнера и содержит плагин Debezium для PostgreSQL.

### Debezium Connector

Файл `connector.json` описывает коннектор с именем `pg-users-orders-connector`.  В конфигурации задаются параметры подключения к базе (`database.hostname`, `database.user`, `database.password`), имя сервера (`database.server.name = demo`) и список таблиц, за которыми нужно следить (`table.include.list = public.users,public.orders`).  Параметр `transforms.unwrap.type` заставляет Debezium удалять обёртку (метаданные CDC), публикуя только фактическое состояние строки.

### Мониторинг (Prometheus + Grafana)

Prometheus с помощью scrape‑конфигурации `kafka-connect` обращается к экспортёру JMX внутри `kafka-connect` на порту `9876`.  Эта точка отдаёт метрики по форматированным правилам из `kafka-connect.yml`.  Grafana использует Prometheus как источник данных и предоставляет готовый дешборд `connect.json`, который отображает количество задач коннектора, состояние задач (running/paused), количество обработанных сообщений и другие показатели.


## ✅ Проверка работоспособности

1. После регистрации коннектора в REST API убедитесь, что его состояние `RUNNING` и количество запущенных задач равно `1`.
2. Сгенерируйте новые события в PostgreSQL — сделайте INSERT/UPDATE/DELETE в таблицах `users` или `orders`. 
3. В Grafana на дешборде `Connect` наблюдайте рост счётчиков обработанных записей и отсутствие ошибок.
4. Запустите consumer для получения сообщений из топика. 

После завершения работы можно остановить контейнеры командой:

```bash
docker compose down
```