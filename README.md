# Developer Metrics Pipeline

Pipeline de coleta e agregação de métricas de produtividade de desenvolvedores, construído em Go com comunicação assíncrona via SQS e persistência no DynamoDB — tudo rodando localmente com LocalStack.

---

## Visão Geral

O sistema recebe eventos brutos de métricas (commits, pull requests, tempo de review), valida e enriquece cada evento, e agrega os totais por desenvolvedor — expondo os resultados via API REST.

```
                  ┌─────────────────────────────────────────────────────┐
                  │                    LocalStack :4566                  │
                  │                                                       │
                  │  ┌─────────────┐          ┌──────────────────────┐  │
                  │  │  raw-events │          │   processed-events   │  │
                  │  │     SQS     │          │        SQS           │  │
                  │  └──────┬──────┘          └──────────┬───────────┘  │
                  │         │                            │               │
                  │  ┌──────▼──────┐                    │               │
                  │  │raw-events   │          ┌──────────▼───────────┐  │
                  │  │   -dlq      │          │ processed-events-dlq │  │
                  │  └─────────────┘          └──────────────────────┘  │
                  │                                                       │
                  │         ┌────────────────────────────────────────┐  │
                  │         │              DynamoDB                  │  │
                  │         │  • events          • developer_summary │  │
                  │         └────────────────────────────────────────┘  │
                  └─────────────────────────────────────────────────────┘
                           ▲                  ▲
                           │                  │
              ┌────────────┴──┐    ┌──────────┴────────────┐
              │   Processor   │    │      Aggregator        │
              │               ├───►│                        │
              │ • valida      │    │ • agrega métricas      │
              │ • enriquece   │    │ • garante idempotência │
              │ • worker pool │    │ • persiste no DynamoDB │
              └───────────────┘    │ • expõe API REST :8080 │
                                   └────────────────────────┘
```

---

## Tecnologias

| | |
|---|---|
| **Linguagem** | Go 1.24 |
| **Mensageria** | AWS SQS (via LocalStack) |
| **Banco de dados** | AWS DynamoDB (via LocalStack) |
| **Infraestrutura local** | LocalStack 3.x |
| **Containers** | Docker + Docker Compose |
| **AWS SDK** | aws-sdk-go-v2 |
| **HTTP** | `net/http` padrão (Go 1.22 routing) |
| **Logs** | `log/slog` (JSON estruturado) |

---

## Arquitetura

O projeto segue **Clean Architecture** em cada serviço, com separação clara entre domínio, casos de uso e infraestrutura:

```
services/
├── processor/
│   └── internal/
│       ├── domain/        # RawEvent, ProcessedEvent, regras de validação
│       ├── usecase/       # Orquestração: validar → enriquecer → publicar
│       └── infra/
│           ├── queue/     # Adapter SQS (consumer + publisher)
│           ├── worker/    # Worker pool concorrente
│           └── config/    # Configuração via variáveis de ambiente
└── aggregator/
    └── internal/
        ├── domain/        # ProcessedEvent, DeveloperSummary, lógica de agregação
        ├── usecase/       # Orquestração: idempotência → persistir → agregar
        └── infra/
            ├── queue/     # Adapter SQS (consumer)
            ├── repository/# Adapters DynamoDB (events + summary)
            ├── api/       # Handlers HTTP
            └── config/    # Configuração via variáveis de ambiente
```

### Fluxo de dados

1. Um evento bruto chega na fila `raw-events`
2. O **Processor** consome via long polling (worker pool configurável)
3. O evento é **validado** (UUID, campos obrigatórios, valores permitidos, timestamp)
4. Se válido: enriquecido com `processed_at` e `processor_id`, publicado em `processed-events`
5. Se inválido: não deletado da fila → SQS reenvia até 3× → vai para `raw-events-dlq`
6. O **Aggregator** consome `processed-events`
7. Checa **idempotência** pelo `event_id` antes de processar
8. Persiste o evento individual na tabela `events`
9. Atualiza incrementalmente a tabela `developer_summary`
10. A **API REST** expõe os dados agregados em tempo real

---

## Como rodar

### Pré-requisitos

- [Docker](https://docs.docker.com/get-docker/) e [Docker Compose](https://docs.docker.com/compose/install/)
- `curl` ou qualquer cliente HTTP para testar a API

### Subindo o ambiente

```bash
# Clonar o repositório
git clone <url-do-repositorio>
cd TesteItau

# Subir todos os serviços (LocalStack + Processor + Aggregator)
docker-compose up --build
```

> **Primeira execução:** o LocalStack leva alguns segundos para criar as filas e tabelas. Os serviços aguardam o LocalStack estar pronto antes de iniciar.

Aguarde as seguintes mensagens nos logs:

```
localstack  | Ready.
processor   | {"level":"INFO","msg":"worker pool started","workers":5}
aggregator  | {"level":"INFO","msg":"server started","addr":":8080"}
```

### Verificando que está tudo ok

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

---

## Testando com dados reais

Com o ambiente rodando, execute o script de seed para enviar eventos de teste para a fila:

```bash
bash scripts/seed.sh
```

O script envia **22 mensagens**:

| Tipo | Quantidade | Descrição |
|------|-----------|-----------|
| Válidas | 17 | Distribuídas entre 4 developers (`dev-42`, `dev-99`, `dev-17`, `dev-55`, `dev-77`) cobrindo os 3 tipos de métrica |
| Inválidas | 3 | `metric_type` inválido, UUID malformado, valor fora do limite — testam a DLQ |
| Duplicadas | 2 | Mesmo `event_id` de mensagens anteriores — testam idempotência |

Após alguns segundos, consulte os resultados:

```bash
# Todos os eventos de um developer
curl http://localhost:8080/metrics/dev-42

# Resumo agregado
curl http://localhost:8080/metrics/dev-42/summary
curl http://localhost:8080/metrics/dev-99/summary
curl http://localhost:8080/metrics/dev-17/summary
curl http://localhost:8080/metrics/dev-55/summary
curl http://localhost:8080/metrics/dev-77/summary
```

---

## API REST

Base URL: `http://localhost:8080`

### `GET /health`

Verifica a conectividade com SQS e DynamoDB.

```bash
curl http://localhost:8080/health
```

```json
{"status": "ok"}
```

Em caso de falha:

```json
{"status": "error", "detail": "dynamodb: connection refused"}
```

---

### `GET /metrics/{developer_id}`

Retorna todos os eventos individuais processados para um desenvolvedor.

```bash
curl http://localhost:8080/metrics/dev-42
```

```json
[
  {
    "event_id": "550e8400-e29b-41d4-a716-446655440001",
    "developer_id": "dev-42",
    "metric_type": "commits",
    "value": 8,
    "repository": "org/backend-api",
    "timestamp": "2026-04-14T09:00:00Z",
    "processed_at": "2026-04-14T09:00:05Z",
    "processor_id": "processor-instance-1"
  },
  ...
]
```

---

### `GET /metrics/{developer_id}/summary`

Retorna o resumo agregado de métricas de um desenvolvedor.

```bash
curl http://localhost:8080/metrics/dev-42/summary
```

```json
{
  "developer_id": "dev-42",
  "total_commits": 142,
  "total_pull_requests": 38,
  "avg_review_time_minutes": 45.2,
  "events_processed": 195,
  "last_activity": "2026-04-15T10:30:00Z"
}
```

Retorna `404` se o developer não tiver nenhum evento processado.

---

## Estrutura dos eventos

### Entrada — fila `raw-events`

```json
{
  "event_id": "uuid-v4",
  "developer_id": "dev-123",
  "metric_type": "commits | pull_requests | review_time_minutes",
  "value": 15,
  "repository": "org/repo-name",
  "timestamp": "2026-04-15T10:30:00Z"
}
```

### Saída — fila `processed-events`

```json
{
  "event_id": "uuid-v4",
  "developer_id": "dev-123",
  "metric_type": "commits",
  "value": 15,
  "repository": "org/repo-name",
  "timestamp": "2026-04-15T10:30:00Z",
  "processed_at": "2026-04-15T10:30:05Z",
  "processor_id": "processor-instance-1"
}
```

### Regras de validação (Processor)

| Campo | Regra |
|-------|-------|
| `event_id` | Obrigatório, UUID v4 válido |
| `developer_id` | Obrigatório, não pode ser vazio |
| `metric_type` | Deve ser `commits`, `pull_requests` ou `review_time_minutes` |
| `value` | Deve ser `>= 0`; para `review_time_minutes` o máximo é `1440` (24h) |
| `timestamp` | Obrigatório, não pode ser uma data futura |

---

## Configuração

Todos os parâmetros são configuráveis via variáveis de ambiente (com defaults razoáveis para desenvolvimento local):

### Processor

| Variável | Default | Descrição |
|----------|---------|-----------|
| `AWS_ENDPOINT` | `http://localstack:4566` | Endpoint do LocalStack |
| `AWS_REGION` | `us-east-1` | Região AWS |
| `RAW_EVENTS_QUEUE` | `raw-events` | Nome da fila de entrada |
| `PROCESSED_EVENTS_QUEUE` | `processed-events` | Nome da fila de saída |
| `WORKER_COUNT` | `5` | Número de workers concorrentes |
| `PROCESSOR_ID` | `processor-instance-1` | Identificador da instância |

### Aggregator

| Variável | Default | Descrição |
|----------|---------|-----------|
| `AWS_ENDPOINT` | `http://localstack:4566` | Endpoint do LocalStack |
| `AWS_REGION` | `us-east-1` | Região AWS |
| `PROCESSED_EVENTS_QUEUE` | `processed-events` | Nome da fila de entrada |
| `EVENTS_TABLE` | `events` | Tabela DynamoDB de eventos |
| `SUMMARY_TABLE` | `developer_summary` | Tabela DynamoDB de resumos |
| `PORT` | `8080` | Porta HTTP |

---

## Estrutura do repositório

```
.
├── docker-compose.yml
├── infra/
│   └── localstack/
│       └── init-aws.sh          # Cria filas SQS e tabelas DynamoDB no startup
├── scripts/
│   └── seed.sh                  # Publica mensagens de teste (válidas + inválidas + duplicadas)
└── services/
    ├── processor/
    │   ├── cmd/main.go
    │   ├── internal/
    │   │   ├── domain/          # Entidades e validações
    │   │   ├── usecase/         # Orquestração de negócio
    │   │   └── infra/           # SQS, worker pool, config
    │   ├── Dockerfile
    │   └── go.mod
    └── aggregator/
        ├── cmd/main.go
        ├── internal/
        │   ├── domain/          # Entidades e lógica de agregação
        │   ├── usecase/         # Orquestração de negócio
        │   └── infra/           # SQS, DynamoDB, HTTP handlers, config
        ├── Dockerfile
        └── go.mod
```

---

## Comandos úteis

```bash
# Subir em background
docker-compose up --build -d

# Ver logs de um serviço específico
docker-compose logs -f processor
docker-compose logs -f aggregator

# Parar tudo
docker-compose down

# Parar e remover volumes (reseta o estado do LocalStack)
docker-compose down -v

# Enviar um evento manualmente para a fila
aws --endpoint-url=http://localhost:4566 sqs send-message \
  --queue-url http://localhost:4566/000000000000/raw-events \
  --message-body '{
    "event_id": "550e8400-e29b-41d4-a716-446655440099",
    "developer_id": "dev-123",
    "metric_type": "commits",
    "value": 5,
    "repository": "org/meu-repo",
    "timestamp": "2026-04-15T10:30:00Z"
  }'

# Consultar o número de mensagens na DLQ (mensagens inválidas)
aws --endpoint-url=http://localhost:4566 sqs get-queue-attributes \
  --queue-url http://localhost:4566/000000000000/raw-events-dlq \
  --attribute-names ApproximateNumberOfMessages

aws --endpoint-url=http://localhost:4566 sqs get-queue-attributes \
  --queue-url http://localhost:4566/000000000000/processed-events-dlq \
  --attribute-names ApproximateNumberOfMessages

```
