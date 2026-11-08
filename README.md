# 🔷 MS1: Jira Issues Service (`jira-issues-go`)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)]()
[![Coverage](https://img.shields.io/badge/Coverage-64.8%25-brightgreen.svg)]()
[![gRPC](https://img.shields.io/badge/gRPC-:50051-244c5a.svg)]()
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-336791?logo=postgresql&logoColor=white)]()
[![Kafka](https://img.shields.io/badge/Kafka-Producer-black?logo=apachekafka)]()

Microservicio de alto rendimiento implementado en **Go (Golang)** responsable del ciclo de vida de **Incidencias (Issues)** y **Proyectos** en la plataforma **Jira AI**. Expone un servidor **gRPC** nativo en el puerto `:50051`, persiste en **PostgreSQL**, y emite eventos de dominio hacia **Apache Kafka** sin dependencias CGO (**Zero-CGO**).

---

## 🏛️ Rol en la Arquitectura e Interoperabilidad

```mermaid
flowchart LR
    BFF["🚀 BFF Gateway\n(NestJS / GraphQL)"] -->|"gRPC :50051\n(GetBacklog, CreateIssue)"| GO["🔷 MS1: Issues\n(Go 1.22+ Server)"]
    S2["🔶 MS2: Sprints\n(NestJS)"] -->|"gRPC :50051\n(BatchAssignToSprint)"| GO
    
    GO --- DB[("PostgreSQL\n(issues_db)")]
    GO -.->|"Publica Eventos\n(tópico: issues.events)"| KAFKA{{"Apache Kafka\n(:9092)"}}
```

### Protocolos de Entrada y Salida (Regla de la Pizarra)
* **Ingress (Entrada):**
  * **gRPC** sobre TCP en el puerto `:50051` implementando el contrato `jira.issues.v1.IssuesService`.
* **Egress (Salida):**
  * **Apache Kafka**: Publica mensajes JSON conformes con el esquema `issues.events.json` en el tópico `issues.events` ante cualquier creación, actualización o cambio de estado de una incidencia.
* **Persistencia:**
  * **PostgreSQL 15** mediante GORM y drivers puros de Go (`pgx`), garantizando transacciones ACID para transiciones de estado y asignación masiva de incidencias a sprints.

---

## 📂 Estructura del Proyecto (Clean Architecture)

```text
jira-issues-go/
├── cmd/
│   └── server/
│       └── main.go               # Punto de entrada del servidor gRPC
├── db/
│   └── migrations/              # Scripts SQL de inicialización y esquemas
├── internal/
│   ├── config/                  # Carga de variables de entorno (12-Factor App)
│   ├── domain/                  # Entidades de dominio, enumeradores y errores
│   │   ├── issue.go             # Entidad Issue con validaciones de negocio
│   │   ├── project.go           # Entidad Project
│   │   ├── priority.go          # Prioridades (LOW, MEDIUM, HIGH, CRITICAL)
│   │   ├── status.go            # Estados (TODO, IN_PROGRESS, IN_REVIEW, DONE)
│   │   └── converter.go         # Mapeo bidireccional Dominio <-> Protobuf
│   ├── grpc/                    # Adaptadores gRPC y registro de servicios
│   │   ├── handler.go           # Implementación de los RPCs de Protobuf
│   │   └── server.go            # Configuración de interceptores y listener
│   ├── kafka/                   # Productor Kafka puro en Go (segmentio/kafka-go)
│   │   ├── event.go             # Estructura del evento de dominio
│   │   └── producer.go          # Productor con reintentos y particionado
│   ├── repository/              # Puertos y adaptadores de persistencia
│   │   └── postgres/            # Implementación GORM / PostgreSQL
│   └── service/                 # Casos de uso de negocio (IssueService)
├── .github/workflows/
│   └── ci.yml                   # Pipeline CI con verificación de cobertura >= 60%
├── Dockerfile                   # Build multi-stage Zero-CGO (distroless/alpine)
├── go.mod
└── go.sum
```

---

## ⚙️ Variables de Entorno

| Variable | Descripción | Valor por Defecto |
| :--- | :--- | :--- |
| `PORT` | Puerto de escucha gRPC | `50051` |
| `DATABASE_URL` | Cadena de conexión PostgreSQL | `postgres://jira_user:jira_password@localhost:5434/issues_db?sslmode=disable` |
| `KAFKA_BROKERS` | Brokers de Apache Kafka | `localhost:9092` |
| `KAFKA_TOPIC_ISSUES` | Tópico de eventos de incidencias | `issues.events` |
| `LOG_LEVEL` | Nivel de logs | `info` |

---

## 🧪 Pruebas Automatizadas y Cobertura

El microservicio cuenta con pruebas unitarias, de concurrencia y de integración con mocks en memoria:

```bash
# Descargar dependencias
go mod download

# Ejecutar suite de pruebas con análisis de cobertura
go test -v -coverprofile=coverage.out ./...

# Visualizar porcentaje de cobertura total en consola
go tool cover -func=coverage.out
```

### Métrica de Cobertura Verificada
* **Cobertura Total:** **64.80%** (supera el umbral mínimo del 60.0%).
* **Pruebas de Concurrencia:** Verificación de bloqueos y condiciones de carrera con `-race`.

---

## 🐳 Despliegue con Docker

El microservicio utiliza una compilación **Multi-stage Zero-CGO** para generar un binario estático ligero y seguro:

```bash
# Compilar la imagen Docker
docker build -t jira-ai-issues-service .

# Ejecutar el contenedor
docker run -d -p 50051:50051 \
  -e DATABASE_URL="postgres://jira_user:jira_password@postgres-issues:5432/issues_db?sslmode=disable" \
  -e KAFKA_BROKERS="kafka:9092" \
  jira-ai-issues-service
```

---

## 📄 Licencia

MIT License © 2026 Jira AI Team.
