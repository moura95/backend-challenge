# Backend Challenge

**API RESTful completa para gestão de usuários com DDD+ Clean Architecture**

## 📋 Descrição

Aplicação desenvolvida seguindo princípios de DDD + Clean Architecture, implementando autenticação JWT, CRUD completo de usuários.
## 🚀 Demo

- **Documentação Swagger**: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- **MailCatcher UI**: [http://localhost:1080](http://localhost:1080) (Interface para emails de desenvolvimento)
- **RabbitMQ Management**: [http://localhost:15672](http://localhost:15672) (user: rabbitmq, pass: rabbitmq)

## 🚀 Recursos

### Backend (Go + Gin)
- ✅ **Clean Architecture** com Domain-Driven Design
- ✅ **JWT/Paseto Authentication** com middleware seguro
- ✅ **CRUD Completo** de usuários com validações
- ✅ **Sistema de Emails Assíncronos** com RabbitMQ
- ✅ **Retry Automático** para emails falhados
- ✅ **Database Migrations** com golang-migrate
- ✅ **SQLC** para type-safe SQL
- ✅ **Testes de Integração** com Testcontainers
- ✅ **Docker Compose** ambiente completo
- ✅ **Swagger Documentation** automática

## 🛠️ Tecnologias

**Backend:**
- Go 1.21+
- Gin Web Framework
- PostgreSQL
- RabbitMQ (Messaging)
- SQLC (Type-safe SQL)
- Paseto (JWT alternative)
- Testcontainers (Testing)
- MailCatcher (Email testing)
- Swagger/OpenAPI

**Arquitetura:**
- Clean Architecture
- Domain-Driven Design
- Repository Pattern
- Use Cases Pattern
- Dependency Injection

## 📋 Pré-requisitos

- Go 1.21 ou superior
- Docker e Docker Compose
- Make (para comandos do Makefile)

### Instalação das Ferramentas

#### Golang-Migrate
```bash
# MacOS
brew install golang-migrate

# Ubuntu/Debian
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/
```

#### SQLC
```bash
# MacOS
brew install sqlc

# Go install
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

## 🚀 Como Rodar

### 📊 Setup Completo (Recomendado)

#### 1. Clone o projeto
```bash
git clone https://github.com/moura95/backend-challenge
cd backend-challenge
```

#### 2. Configure o ambiente
```bash
cp .env.example .env
```

#### 3. Inicie o ambiente completo
```bash
# Inicia PostgreSQL + RabbitMQ + MailCatcher + Migrations + API
make start
```

#### 4. Acesse os serviços
- **API**: http://localhost:8080
- **Swagger**: http://localhost:8080/swagger/index.html
- **MailCatcher**: http://localhost:1080
- **RabbitMQ**: http://localhost:15672

### 🛠️ Desenvolvimento Manual

```bash
make up

# 2. Execute as migrations
make migrate-up

# 3. Rode a aplicação
make run
```

## 🧪 Testes

```bash
# Rodar todos os testes
make test

# Testes do domain apenas
make test-domain

# Pipeline completo (fmt + vet + test + build)
make ci
```

## 📚 Endpoints da API

### 🔐 Autenticação
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `POST` | `/api/auth/signup` | Criar nova conta |
| `POST` | `/api/auth/signin` | Login do usuário |

### 👤 Usuários (Autenticado)
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET` | `/api/account/me` | Perfil do usuário |
| `PUT` | `/api/account/me` | Atualizar perfil |
| `DELETE` | `/api/account/me` | Deletar conta |
| `GET` | `/api/users` | Listar usuários (paginado) |

### ℹ️ Sistema
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET` | `/healthz` | Health check |

## 💡 Exemplos de Uso

### Criar Conta
```bash
curl -X POST http://localhost:8080/api/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "name": "João Silva",
    "email": "joao@example.com",
    "password": "senha123"
  }'
```

### Login
```bash
curl -X POST http://localhost:8080/api/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "email": "joao@example.com",
    "password": "senha123"
  }'
```

### Buscar Perfil (Autenticado)
```bash
curl -X GET http://localhost:8080/api/account/me \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Listar Usuários com Busca
```bash
curl "http://localhost:8080/api/users?page=1&page_size=10&search=João" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## 🏗️ Regras de Negócio

### 🔒 Autenticação
- **JWT/Paseto tokens** com expiração de 24h
- **Passwords** hasheados com bcrypt
- **Middleware** de autenticação em rotas protegidas

### 👥 Usuários
- **Email único** por usuário
- **Nome** mínimo 2 caracteres, máximo 100
- **Senha** mínimo 6 caracteres
- **Validação de email** formato RFC compliant

### 📧 Sistema de Emails
- **Email de boas-vindas** automático no signup
- **Processamento assíncrono** via RabbitMQ
- **Templates HTML** responsivos

### 📊 Paginação
- **Página padrão**: 1
- **Tamanho padrão**: 10 itens
- **Máximo**: 100 itens por página
- **Busca**: por nome ou email

## 🏛️ Arquitetura

O projeto segue **Clean Architecture** com separação clara de responsabilidades:

```
cmd/                              # Entry points
├── main.go                      # Application startup

internal/
├── application/                 # Application Layer
│   └── usecases/               # Use Cases (Business Logic)
│       ├── auth/               # Authentication use cases
│       ├── user/               # User management use cases
│       └── email/              # Email processing use cases
│
├── domain/                     # Domain Layer (Business Rules)
│   ├── user/                   # User entity & business rules
│   └── email/                  # Email entity & business rules
│
├── infra/                      # Infrastructure Layer
│   ├── database/               # Database setup & migrations
│   ├── repository/             # Data access implementations
│   ├── security/               # JWT, crypto, hashing
│   ├── messaging/              # RabbitMQ implementation
│   ├── email/                  # SMTP service
│   └── http/                   # HTTP server setup
│
└── interfaces/                 # Interface Adapters
    └── http/                   # HTTP handlers & middleware
        ├── handlers/           # HTTP request handlers
        ├── middlewares/        # Authentication, CORS, etc
        └── ginx/              # HTTP utilities

deployments/                    # Deployment configurations
├── docker-compose/            # Docker Compose setup
└── build/                     # Dockerfile

tests/                         # Test organization
├── domain/                    # Domain tests
├── integration/               # Integration tests
└── load/                      # Load testing (Artillery)
```

### 🎯 Camadas da Clean Architecture

1. **Domain Layer** - Entidades e regras de negócio puras
2. **Application Layer** - Use Cases e orquestração
3. **Infrastructure Layer** - Implementações técnicas (DB, HTTP, etc)
4. **Interface Adapters** - Adaptadores para frameworks externos


## 🔧 Comandos Úteis

```bash
# Desenvolvimento
make run              # Roda a aplicação
make start            # Setup completo (up + migrate + run)
make restart          # Reinicia ambiente completo

# Docker
make up               # Sobe infraestrutura (DB + RabbitMQ + MailCatcher)
make down             # Para e limpa ambiente

# Database
make migrate-up       # Aplica migrations
make migrate-down     # Reverte migrations
make sqlc             # Gera código SQLC

# Testes
make test             # Todos os testes
make test-domain      # Apenas domain tests
make test-coverage    # Coverage com HTML

# Code Quality
make fmt              # Formata código
make vet              # Análise estática
make ci               # Pipeline completo

# Documentação
make swag             # Gera Swagger docs

# Utilitários
make mailcatcher      # Abre MailCatcher UI
make rabbitmq         # Abre RabbitMQ Management
```

## 🚀 Features Avançadas

### 🔒 Segurança
- **Paseto tokens** (mais seguro que JWT)
- **Password hashing** com bcrypt + salt
- **Input validation** em todas as camadas
- 
### 📈 Performance
- **Connection pooling** para database
- **Async email processing** com RabbitMQ
- **Testcontainers** para testes isolados
- **Type-safe SQL** com SQLC

### 🛡️ Observabilidade
- **Structured logging** com contexto
- **Health checks** para monitoramento
- **Error tracking** com stack traces
- 
### 🧪 Testing
- **Unit tests** para domain logic
- **Integration tests** com Testcontainers
- **Repository tests** contra PostgreSQL real
- **Use case tests** com Testcontainers

## 👨‍💻 Autor

**Seu Nome** - *Backend Engineer*
- GitHub: [@moura95](https://github.com/moura95)
- LinkedIn: [Guilherme Moura](https://www.linkedin.com/in/guilherme-moura95/)
- Email: junior.moura19@hotmail.com
