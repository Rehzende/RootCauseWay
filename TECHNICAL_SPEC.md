# RootCauseway — Especificação Técnica
## Root Cause Analysis Intelligence — Documento de Referência Técnica

> **Versão:** 1.0 | **Data:** 2026-06-30
> Baseado exclusivamente no inventário de funcionalidades do arquivo `FEATURES.md`.
> Não contém funcionalidades inferidas ou hipotéticas.

---

## Sumário

- [1. Visão Geral da Plataforma](#1-visão-geral-da-plataforma)
  - [1.1 Propósito](#11-propósito)
  - [1.2 Stack Tecnológico](#12-stack-tecnológico)
  - [1.3 Princípios Arquiteturais](#13-princípios-arquiteturais)
  - [1.4 Diagrama de Arquitetura](#14-diagrama-de-arquitetura)
- [2. Ingestão e Normalização de Alertas](#2-ingestão-e-normalização-de-alertas)
  - [2.1 Endpoint de Ingestão](#21-endpoint-de-ingestão)
  - [2.2 Fontes Suportadas](#22-fontes-suportadas)
  - [2.3 Modelo NormalizedAlert](#23-modelo-normalizedalert)
  - [2.4 Alert Snapshot](#24-alert-snapshot)
  - [2.5 Quarentena de Alertas](#25-quarentena-de-alertas)
  - [2.6 Fluxo Completo de Ingestão](#26-fluxo-completo-de-ingestão)
- [3. Gestão de Incidentes](#3-gestão-de-incidentes)
  - [3.1 Ciclo de Vida do Incidente](#31-ciclo-de-vida-do-incidente)
  - [3.2 Modelo Incident](#32-modelo-incident)
  - [3.3 Timeline de Eventos](#33-timeline-de-eventos)
  - [3.4 Coleta de Evidências](#34-coleta-de-evidências)
  - [3.5 Incident Cockpit](#35-incident-cockpit)
  - [3.6 DAG de Execução de Agentes](#36-dag-de-execução-de-agentes)
  - [3.7 Endpoints de Incidentes](#37-endpoints-de-incidentes)
- [4. Pipeline de Investigação com IA](#4-pipeline-de-investigação-com-ia)
  - [4.1 Visão Geral do Pipeline](#41-visão-geral-do-pipeline)
  - [4.2 Estágio 1 — Triage](#42-estágio-1--triage)
  - [4.3 Estágio 2 — Evidence](#43-estágio-2--evidence)
  - [4.4 Estágio 3 — Hypothesis](#44-estágio-3--hypothesis)
  - [4.5 Estágio 4 — RCI (Root Cause Investigation)](#45-estágio-4--rci-root-cause-investigation)
  - [4.6 Estágio 5 — RCA (Root Cause Analysis)](#46-estágio-5--rca-root-cause-analysis)
  - [4.7 Estágio 6 — Postmortem](#47-estágio-6--postmortem)
  - [4.8 Decisões do Orquestrador](#48-decisões-do-orquestrador)
  - [4.9 Endpoints de Análise](#49-endpoints-de-análise)
- [5. Framework de Agentes (A2A)](#5-framework-de-agentes-a2a)
  - [5.1 Protocolo Google A2A](#51-protocolo-google-a2a)
  - [5.2 Agent Card](#52-agent-card)
  - [5.3 Ciclo de Vida de uma A2A Task](#53-ciclo-de-vida-de-uma-a2a-task)
  - [5.4 Modelo de Hospedagem Híbrida](#54-modelo-de-hospedagem-híbrida)
  - [5.5 Autenticação de Agentes](#55-autenticação-de-agentes)
  - [5.6 Endpoints A2A](#56-endpoints-a2a)
- [6. Registro de Skills](#6-registro-de-skills)
  - [6.1 Definição de Skill](#61-definição-de-skill)
  - [6.2 Categorias de Skills](#62-categorias-de-skills)
  - [6.3 Mapeamento Agente-Skill](#63-mapeamento-agente-skill)
  - [6.4 Endpoints de Skills](#64-endpoints-de-skills)
- [7. Credenciais e Acesso JIT](#7-credenciais-e-acesso-jit)
  - [7.1 Provedores de Credencial](#71-provedores-de-credencial)
  - [7.2 Tipos de Recursos](#72-tipos-de-recursos)
  - [7.3 Fluxo de Leasing JIT](#73-fluxo-de-leasing-jit)
  - [7.4 Access Policies](#74-access-policies)
  - [7.5 Endpoints de Credenciais](#75-endpoints-de-credenciais)
- [8. Runbooks](#8-runbooks)
  - [8.1 Definição de Runbook](#81-definição-de-runbook)
  - [8.2 Tipos de Steps](#82-tipos-de-steps)
  - [8.3 Execução de Runbook](#83-execução-de-runbook)
  - [8.4 Fluxo de Execução](#84-fluxo-de-execução)
  - [8.5 Endpoints de Runbooks](#85-endpoints-de-runbooks)
- [9. Base de Conhecimento e Loop de Feedback](#9-base-de-conhecimento-e-loop-de-feedback)
  - [9.1 Knowledge Base](#91-knowledge-base)
  - [9.2 Feedback Humano](#92-feedback-humano)
  - [9.3 Similar Incident Matching](#93-similar-incident-matching)
  - [9.4 Correlation Rules](#94-correlation-rules)
  - [9.5 Endpoints](#95-endpoints)
- [10. Fontes de Observabilidade](#10-fontes-de-observabilidade)
  - [10.1 Fontes Suportadas](#101-fontes-suportadas)
  - [10.2 Snapshot Configs](#102-snapshot-configs)
  - [10.3 Endpoints de Observabilidade](#103-endpoints-de-observabilidade)
- [11. Eventos de Mudança](#11-eventos-de-mudança)
  - [11.1 Modelo ChangeEvent](#111-modelo-changeevent)
  - [11.2 Endpoints](#112-endpoints)
- [12. Notificações e Escalonamento](#12-notificações-e-escalonamento)
  - [12.1 Canais de Notificação](#121-canais-de-notificação)
  - [12.2 Políticas de Escalonamento](#122-políticas-de-escalonamento)
  - [12.3 Auditoria de Notificações](#123-auditoria-de-notificações)
  - [12.4 Endpoints](#124-endpoints)
- [13. Analytics](#13-analytics)
  - [13.1 Métricas Disponíveis](#131-métricas-disponíveis)
  - [13.2 Detalhamento de MTTR](#132-detalhamento-de-mttr)
  - [13.3 Efetividade dos Agentes](#133-efetividade-dos-agentes)
  - [13.4 Endpoints de Analytics](#134-endpoints-de-analytics)
- [14. Marketplace de Agentes](#14-marketplace-de-agentes)
  - [14.1 Modelo MarketplaceAgent](#141-modelo-marketplaceagent)
  - [14.2 Endpoints do Marketplace](#142-endpoints-do-marketplace)
- [15. Catálogo de Software](#15-catálogo-de-software)
  - [15.1 Modelo SoftwareEntry](#151-modelo-softwareentry)
  - [15.2 Uso do Catálogo na Investigação](#152-uso-do-catálogo-na-investigação)
  - [15.3 Endpoints](#153-endpoints)
- [16. Autenticação e Autorização](#16-autenticação-e-autorização)
  - [16.1 Métodos de Autenticação](#161-métodos-de-autenticação)
  - [16.2 RBAC — Roles e Permissões](#162-rbac--roles-e-permissões)
  - [16.3 API Keys](#163-api-keys)
  - [16.4 Gerenciamento de Sessão](#164-gerenciamento-de-sessão)
  - [16.5 Fluxo de Autenticação SSO](#165-fluxo-de-autenticação-sso)
  - [16.6 Endpoints de Auth](#166-endpoints-de-auth)
- [17. Comunicação em Tempo Real — WebSocket](#17-comunicação-em-tempo-real--websocket)
  - [17.1 Endpoint WebSocket](#171-endpoint-websocket)
  - [17.2 Eventos Redis Pub/Sub](#172-eventos-redis-pubsub)
  - [17.3 Fluxo de Mensagens](#173-fluxo-de-mensagens)
- [18. Log de Auditoria](#18-log-de-auditoria)
  - [18.1 Modelo AuditLog](#181-modelo-auditlog)
  - [18.2 Endpoint](#182-endpoint)
- [19. CLI](#19-cli)
  - [19.1 Comandos Disponíveis](#191-comandos-disponíveis)
- [20. Frontend — Páginas e Componentes](#20-frontend--páginas-e-componentes)
  - [20.1 Páginas](#201-páginas)
  - [20.2 Componentes de UI](#202-componentes-de-ui)
  - [20.3 Hooks de Tempo Real](#203-hooks-de-tempo-real)
- [21. Referência Completa da API](#21-referência-completa-da-api)
  - [21.1 Autenticação da API](#211-autenticação-da-api)
  - [21.2 Todos os Endpoints por Domínio](#212-todos-os-endpoints-por-domínio)
  - [21.3 Contagem por Domínio](#213-contagem-por-domínio)
- [22. Infraestrutura e Deploy](#22-infraestrutura-e-deploy)
  - [22.1 Ambientes](#221-ambientes)
  - [22.2 Variáveis de Ambiente](#222-variáveis-de-ambiente)
  - [22.3 Histórico de Migrações do Banco de Dados](#223-histórico-de-migrações-do-banco-de-dados)
  - [22.4 Middleware de Segurança da API](#224-middleware-de-segurança-da-api)
  - [22.5 Testes](#225-testes)
  - [22.6 Kubernetes](#226-kubernetes)

---

## 1. Visão Geral da Plataforma

### 1.1 Propósito

O RootCauseway (Root Cause Analysis Intelligence) é uma plataforma multi-tenant de gestão de incidentes orientada a IA. Seu objetivo é automatizar o ciclo de vida completo de investigação de incidentes — desde a ingestão de alertas até a geração de postmortem sem culpabilização — por meio de um pipeline sequencial de agentes de IA especializados.

### 1.2 Stack Tecnológico

| Camada | Tecnologia | Porta |
|---|---|---|
| Frontend | React + Vite + TailwindCSS | 3000 (dev) / 80 (prod) |
| Backend API | Go (Gin) | 8080 |
| Orquestrador de Agentes | Python (FastAPI) | 8081 |
| Agente de Triage | Python (microsserviço A2A) | 8090 |
| Agente de Evidence | Python (microsserviço A2A) | 8091 |
| Agente de RCA | Python (microsserviço A2A) | 8092 |
| Agente de Postmortem | Python (microsserviço A2A) | 8093 |
| Banco de Dados | PostgreSQL 17 | 5432 |
| Message Broker | Redis | 6379 |

### 1.3 Princípios Arquiteturais

| Princípio | Descrição |
|---|---|
| **Event-driven** | Redis pub/sub desacopla a ingestão de alertas da orquestração de agentes |
| **A2A Protocol** | Comunicação entre agentes via padrão Google Agent-to-Agent |
| **Multi-tenant** | Isolamento completo por organização em todos os recursos |
| **Contract-first** | Contratos OpenAPI + schemas de eventos Redis definidos em `/contracts/` |
| **Hybrid hosting** | Suporte a agentes gerenciados pelo RootCauseway (Managed) e agentes hospedados pelo cliente (BYOA) |

### 1.4 Diagrama de Arquitetura

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           CLIENTE / BROWSER                             │
│                    React SPA (Vite + TailwindCSS)                       │
│                          porta 3000 / 80                                │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │ HTTP REST + WebSocket
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          BACKEND API (Go/Gin)                           │
│                              porta 8080                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ Auth/RBAC    │  │ Incidents    │  │ Agents/Skills│  │ Analytics  │ │
│  │ Middleware   │  │ Runbooks     │  │ Marketplace  │  │ Audit Log  │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
└──────┬──────────────────────┬────────────────────────────────┬──────────┘
       │                      │ Redis Events                   │
       ▼                      ▼                                ▼
┌──────────────┐   ┌──────────────────────┐        ┌──────────────────────┐
│  PostgreSQL  │   │   REDIS (pub/sub)    │        │  Blob Storage        │
│  (porta 5432)│   │   (porta 6379)       │        │  (evidências)        │
└──────────────┘   └──────────┬───────────┘        └──────────────────────┘
                              │ alert.received / events
                              ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    ORQUESTRADOR (Python/FastAPI)                         │
│                              porta 8081                                 │
│     AlertWorker ──► Orchestrator (LLM decide skills) ──► A2AClient ──►  │
│     EventPublisher                                                       │
└──────┬──────────────────────────────────────────────────────────────────┘
       │ A2A Protocol (HTTP)
       ├──────────────────────────┐
       ▼                          ▼
┌──────────────┐        ┌─────────────────────────────────────────────────┐
│ Triage Agent │        │ Evidence Agent │ RCA Agent │ Postmortem Agent   │
│  porta 8090  │        │  porta 8091    │ porta 8092│   porta 8093       │
└──────────────┘        └─────────────────────────────────────────────────┘
                                         ▲
                        Agentes externos (BYOA) via endpoint registrado
```

---

## 2. Ingestão e Normalização de Alertas

### 2.1 Endpoint de Ingestão

Cada entrada no catálogo de software recebe um token único de ingestão. Sistemas externos enviam alertas para:

```
POST /api/v1/ingest/:token
```

Este é o único endpoint público da plataforma (não requer autenticação JWT). A autenticação é feita via `token` na rota.

### 2.2 Fontes Suportadas

| Fonte | Descrição |
|---|---|
| Datadog | Alertas e monitors do Datadog |
| Prometheus AlertManager | Alertas do AlertManager |
| Grafana | Alertas do Grafana |
| OpenTelemetry Collector | Eventos OTel |
| Custom | Qualquer payload JSON arbitrário |

### 2.3 Modelo NormalizedAlert

Todos os payloads de entrada são normalizados em uma estrutura fonte-agnóstica:

| Campo | Tipo | Descrição |
|---|---|---|
| `title` | string | Título do alerta |
| `description` | string | Descrição completa |
| `severity` | enum | `critical` / `high` / `medium` / `low` |
| `source` | string | Sistema de origem |
| `labels` | map[string]string | Tags chave-valor |
| `annotations` | map[string]string | Metadados estendidos |
| `fingerprint` | string | Hash para deduplicação |
| `starts_at` | timestamp | Horário de início do alerta |
| `software_id` | uuid | Entrada vinculada no catálogo de software |

### 2.4 Alert Snapshot

No momento da ingestão, a plataforma captura um `AlertSnapshot` — uma cópia point-in-time do payload bruto original. Este snapshot é preservado para uso forense durante a fase de análise de evidências.

### 2.5 Quarentena de Alertas

Alertas que não podem ser associados a nenhum software cadastrado são colocados em **quarentena** em vez de serem descartados silenciosamente.

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/quarantine` | GET | Listar alertas em quarentena |
| `/api/v1/quarantine/:id/resolve` | POST | Resolver ou descartar alerta |

A página `QuarantinePage` no frontend exibe os alertas não associados com contexto para triage manual.

### 2.6 Fluxo Completo de Ingestão

```
Sistema externo (Datadog/Prometheus/etc.)
        │
        │ POST /api/v1/ingest/:token
        ▼
┌──────────────────────────────────────────────┐
│              Backend API                      │
│  1. Valida token → identifica software_id     │
│  2. Normaliza payload → NormalizedAlert        │
│  3. Captura AlertSnapshot                      │
│  4. software_id encontrado?                    │
│        SIM ──► publica alert.received (Redis) │
│        NÃO ──► move para Quarentena           │
└──────────────────┬───────────────────────────┘
                   │ Redis pub: alert.received
                   ▼
        Orquestrador (AlertWorker)
        inicia pipeline de investigação
```

---

## 3. Gestão de Incidentes

### 3.1 Ciclo de Vida do Incidente

```
     ┌─────┐     ┌──────────────┐     ┌──────────┐     ┌──────────┐     ┌────────┐
     │ open│────►│ investigating│────►│mitigated │────►│ resolved │────►│ closed │
     └─────┘     └──────────────┘     └──────────┘     └──────────┘     └────────┘
                        │                                     ▲
                        │       (pode voltar se mitigação     │
                        └─────── não for suficiente) ─────────┘
```

Transições de estado geram eventos do tipo `status_changed` na timeline do incidente.

### 3.2 Modelo Incident

| Campo | Tipo | Descrição |
|---|---|---|
| `title` | string | Título do incidente |
| `description` | string | Descrição completa |
| `severity` | enum | `critical` / `high` / `medium` / `low` |
| `status` | enum | `open` / `investigating` / `mitigated` / `resolved` / `closed` |
| `assignee_id` | uuid | Usuário responsável |
| `software_id` | uuid | Software afetado |
| `root_cause` | string | Resumo da causa raiz |
| `mitigation` | string | Passos de mitigação tomados |
| `resolution` | string | Resolução final |
| `resolved_at` | timestamp | Timestamp da resolução |

### 3.3 Timeline de Eventos

Toda ação em um incidente é registrada como um evento timestamped. O conjunto completo de tipos de eventos:

| Tipo de Evento | Descrição |
|---|---|
| `alert_received` | Alerta que disparou o incidente |
| `triage_started` | Início da fase de triage |
| `triage_completed` | Conclusão da fase de triage |
| `evidence_collected` | Artefato de evidência adicionado |
| `hypothesis_generated` | Hipótese produzida |
| `status_changed` | Mudança de estado no ciclo de vida |
| `comment` | Comentário humano |
| `agent_action` | Ação executada por agente de IA |
| `rci_created` | RCI gerada |
| `rca_created` | RCA gerada |
| `postmortem_created` | Postmortem gerado |

### 3.4 Coleta de Evidências

Evidências são tipadas e rastreáveis. Tipos suportados:

| Tipo | Exemplos |
|---|---|
| `log` | Linhas de log, exportações de arquivos de log |
| `metric` | Dados de séries temporais, gráficos |
| `trace` | Trace distribuído |
| `snapshot` | Snapshot automático de observabilidade |
| `agent_output` | Output produzido por um agente de IA |
| `manual` | Notas enviadas por humanos |
| `screenshot` | Screenshots de dashboards |
| `dashboard` | Link/embed de dashboard |
| `config` | Arquivos de configuração |
| `heap_dump` | Memory dumps |

Upload de arquivos é suportado com integração a blob storage:

```
POST /api/v1/incidents/:id/evidence/upload
```

### 3.5 Incident Cockpit

O endpoint de visão completa retorna o contexto enriquecido do incidente em uma única chamada:

```
GET /api/v1/incidents/:id/full
```

Retorna:
- Metadados do incidente
- Todos os eventos (timeline)
- Todas as evidências
- Agent runs (DAG)
- Resultados de RCI, RCA e Postmortem
- Decisões do orquestrador

### 3.6 DAG de Execução de Agentes

Cada fase da investigação é rastreada como um nó em um Grafo Acíclico Dirigido (DAG). Cada nó `AgentRun` registra:

| Campo | Descrição |
|---|---|
| Agent usado | Qual agente executou |
| Status | Estado da execução |
| Input / Output | Dados de entrada e saída |
| Model | Modelo LLM utilizado |
| Tokens consumidos | Custo de tokens |
| Duração | Tempo de execução |
| Mensagem de erro | Se falhou |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/dag` | GET | DAG completo da investigação |
| `/api/v1/incidents/:id/runs` | GET | Listar todas as execuções |
| `/api/v1/incidents/:id/runs/:runId` | GET | Detalhes de uma execução |
| `/api/v1/incidents/:id/runs/:runId/rerun` | POST | Reexecutar agente com falha |

### 3.7 Endpoints de Incidentes

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents` | GET | Listar incidentes (com filtros: status, severity, software) |
| `/api/v1/incidents/:id` | GET | Detalhes do incidente |
| `/api/v1/incidents/:id` | PATCH | Atualizar incidente |
| `/api/v1/incidents/:id/events` | POST | Adicionar evento à timeline |
| `/api/v1/incidents/:id/evidence` | POST | Adicionar evidência |
| `/api/v1/incidents/:id/evidence/upload` | POST | Upload de arquivo de evidência |
| `/api/v1/incidents/:id/full` | GET | Cockpit completo do incidente |
| `/api/v1/incidents/:id/dag` | GET | DAG de investigação |
| `/api/v1/incidents/:id/runs` | GET | Listar agent runs |
| `/api/v1/incidents/:id/runs/:runId` | GET | Detalhes de agent run |
| `/api/v1/incidents/:id/runs/:runId/rerun` | POST | Reexecutar agente |

---

## 4. Pipeline de Investigação com IA

### 4.1 Visão Geral do Pipeline

Não existe um número fixo de estágios nem uma ordem pré-definida. Ao receber
`alert.received`, o orquestrador (Python/FastAPI) monta o contexto do
incidente (software afetado, incidentes similares, evidência já coletada) e
usa um LLM para decidir, a cada chamada, **qual skill chamar em seguida e se
mais alguma é necessária** — o resultado de cada chamada entra no contexto
acumulado que embasa a próxima decisão. Duas execuções do mesmo tipo de
alerta podem despachar conjuntos de skills diferentes (ex.: um incidente
rotulado como Kubernetes puxa `k8s-debug`/`k8s-logs`; um em nuvem Azure pode
puxar skills `azure-*`); nada disso é hardcoded em sequência.

```
Alerta Recebido
        │
        ▼
┌───────────────────────────────────────────────────────────────────┐
│  ORQUESTRADOR — LLM decide a próxima skill a chamar, com base no   │
│  contexto acumulado até aqui (repete até decidir que já é          │
│  suficiente ou até um limite de iterações)                         │
└────────────────────────────┬──────────────────────────────────────┘
                             │  cada chamada acrescenta seu resultado
                             │  ao contexto da próxima decisão
                             ▼
        (exemplos de skills disponíveis, chamadas 0-N vezes cada,
         em qualquer ordem que o LLM decidir)
┌─────────────┐ ┌───────────────┐ ┌───────────┐ ┌────────────┐
│   Triage    │ │   Evidence    │ │    RCA    │ │ Postmortem │  ...e outras
│ (porta 8090)│ │ (porta 8091)  │ │(porta 8092)│ │(porta 8093)│  (k8s-debug,
└─────────────┘ └───────────────┘ └───────────┘ └────────────┘  azure-*, ...)
```

O agente de RCA (porta 8092) é quem produz hipótese, RCI e RCA numa única
chamada — não são três estágios internos separados, são três artefatos de
uma resposta só, que o orquestrador então persiste individualmente via
`IncidentRCI`/`IncidentRCA`.

### 4.2 Triage

**Agente:** Triage Agent (porta 8090)

**Output produzido:**
- Classificação de severidade: `critical` / `high` / `medium` / `low`
- Categoria do problema: `infrastructure` / `application` / `database` / `network` / `security`
- Componentes afetados identificados
- Score de confiança (0–1)

### 4.3 Evidence

**Agente:** Evidence Agent (porta 8091)

**Output produzido:**
- Fontes de dados recomendadas
- Queries de log com justificativa
- Prioridade de coleta por tipo de evidência: `high` / `medium` / `low`
- Tipos de evidência a coletar

### 4.4 RCA (hipótese + RCI + RCA)

**Agente:** RCA Agent (porta 8092)

Uma única chamada a esse agente devolve três artefatos juntos: a hipótese de
causa raiz (estruturada, com ações de investigação recomendadas), o
`IncidentRCI` e o `IncidentRCA` abaixo — o orquestrador extrai e persiste
cada um separadamente.

**Modelo de output `IncidentRCI`:**

| Campo | Tipo | Descrição |
|---|---|---|
| `investigation_summary` | string | Narrativa descritiva da investigação |
| `impact_assessment` | string | Impacto técnico e de negócio |
| `affected_services` | []string | Lista de serviços impactados |
| `affected_users_estimate` | int | Estimativa de usuários afetados |
| `detection_method` | string | Como o problema foi detectado |
| `time_to_detect` (TTD) | duration | Tempo entre início e detecção |
| `acknowledgment_time` | duration | Tempo para reconhecimento |
| `evidence_ids` | []uuid | Referências a artefatos de evidência |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/rci` | POST | Criar RCI |
| `/api/v1/incidents/:id/rci` | GET | Obter RCI |
| `/api/v1/incidents/:id/rci` | PATCH | Atualizar RCI |

### 4.6 Estágio 5 — RCA (Root Cause Analysis)

**Modelo de output `IncidentRCA`:**

| Campo | Tipo | Descrição |
|---|---|---|
| `root_cause_summary` | string | Declaração concisa da causa raiz |
| `root_cause_category` | enum | `infrastructure` / `code` / `configuration` / `dependency` / `human_error` / `capacity` / `security` |
| `contributing_factors` | []string | Lista de causas contribuintes |
| `five_whys` | []string | Breakdown estruturado dos cinco porquês |
| `confidence_score` | float | Confiança da IA (0–1) |
| `evidence_references` | []uuid | Evidências de suporte |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/rca` | POST | Criar RCA |
| `/api/v1/incidents/:id/rca` | GET | Obter RCA |
| `/api/v1/incidents/:id/rca` | PATCH | Atualizar RCA |

### 4.5 Postmortem

**Agente:** Postmortem Agent (porta 8093)

**Modelo de output `IncidentPostmortem`:**

| Campo | Tipo | Descrição |
|---|---|---|
| `title` | string | Título do postmortem |
| `executive_summary` | string | Resumo executivo para stakeholders |
| `incident_timeline` | string | Narrativa cronológica dos eventos |
| `root_cause_detail` | string | Explicação detalhada da causa raiz |
| `impact_detail` | string | Descrição completa do impacto |
| `lessons_learned` | []string | Aprendizados principais |
| `what_went_well` | []string | O que funcionou bem |
| `what_went_wrong` | []string | Áreas de falha |
| `action_items` | []ActionItem | Itens de ação priorizados com responsáveis |
| `prevention_measures` | []string | Medidas para prevenir recorrência |
| `published` | bool | Status de publicação |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/postmortem` | POST | Criar postmortem |
| `/api/v1/incidents/:id/postmortem` | GET | Obter postmortem |
| `/api/v1/incidents/:id/postmortem` | PATCH | Atualizar postmortem |

### 4.6 Decisões do Orquestrador

O orquestrador inteligente registra cada decisão de roteamento:

| Campo | Descrição |
|---|---|
| Tipo de decisão | Qual tipo de roteamento foi tomado |
| Reasoning | Justificativa textual |
| Agentes selecionados | Quais agentes foram escolhidos |
| Contexto utilizado | Dados que embasaram a decisão |
| Confidence | Nível de confiança da decisão |

```
GET /api/v1/incidents/:id/orchestrator/decisions
```

### 4.7 Endpoints de Análise

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/rci` | POST/GET/PATCH | RCI |
| `/api/v1/incidents/:id/rca` | POST/GET/PATCH | RCA |
| `/api/v1/incidents/:id/postmortem` | POST/GET/PATCH | Postmortem |
| `/api/v1/incidents/:id/orchestrator/decisions` | GET | Decisões do orquestrador |

---

## 5. Framework de Agentes (A2A)

### 5.1 Protocolo Google A2A

Todos os agentes implementam o **Google Agent-to-Agent (A2A) Protocol**, que define:
- Dispatch padronizado de tasks
- Input/output como artefatos estruturados
- Advertisement de capacidades via **Agent Cards**
- Health checking

### 5.2 Agent Card

Cada agente publica um Agent Card descrevendo suas capacidades:

| Campo | Descrição |
|---|---|
| Nome, descrição, versão | Identificação do agente |
| Tipos de task suportados | O que o agente pode executar |
| Especificações de input/output | Schemas de dados |
| Skills requeridas | Dependências de skills |
| Requisitos de recursos | CPU, memória, etc. |
| Requisitos de autenticação | Tipo de auth exigido |

```
GET /api/v1/a2a/agents/:id/card
```

### 5.3 Ciclo de Vida de uma A2A Task

```
┌───────────┐    dispatch     ┌─────────┐    execução    ┌───────────┐
│ submitted │───────────────►│ running │───────────────►│ completed │
└───────────┘                └─────────┘                └───────────┘
                                  │                            
                                  │ falha                  ┌────────┐
                                  ├──────────────────────►│ failed │
                                  │                        └────────┘
                                  │ cancelamento           ┌──────────┐
                                  └──────────────────────►│cancelled │
                                                           └──────────┘
```

Cada `A2ATask` registra:

| Campo | Descrição |
|---|---|
| `task_type` | Tipo da task |
| `priority` | Prioridade de execução |
| `input_message` | Input estruturado |
| `output_artifacts` | Artefatos produzidos |
| `orchestrator_reasoning` | Justificativa do orquestrador |
| `dependencies` | Tasks das quais depende |
| `submitted_at` | Timestamp de submissão |
| `started_at` | Timestamp de início |
| `completed_at` | Timestamp de conclusão |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/a2a/tasks` | GET | Listar tasks A2A do incidente |
| `/api/v1/incidents/:id/a2a/tasks` | POST | Dispatchar nova task |
| `/api/v1/incidents/:id/a2a/tasks/:taskId` | GET | Detalhes da task |
| `/api/v1/incidents/:id/a2a/tasks/:taskId` | PATCH | Atualizar status da task |

### 5.4 Modelo de Hospedagem Híbrida

| Modo | Descrição |
|---|---|
| **Managed** | RootCauseway hospeda o container do agente; cliente fornece API keys |
| **BYOA** (Bring Your Own Agent) | Cliente hospeda o próprio agente; registra a URL do endpoint no RootCauseway |

Em ambos os modos, as credenciais são roteadas pelo vault JIT, garantindo que agentes nunca armazenem segredos de longa duração.

### 5.5 Autenticação de Agentes

| Tipo | Descrição |
|---|---|
| Bearer token | Token JWT ou opaco |
| API key | Chave de API no header |
| mTLS | Mutual TLS com certificados |

### 5.6 Endpoints A2A

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/a2a/agents` | GET | Listar todos os agentes A2A |
| `/api/v1/a2a/agents` | POST | Registrar novo agente |
| `/api/v1/a2a/agents/:id` | GET | Detalhes do agente |
| `/api/v1/a2a/agents/:id` | PUT | Atualizar agente |
| `/api/v1/a2a/agents/:id` | DELETE | Remover agente |
| `/api/v1/a2a/agents/:id/card` | GET | Agent Card |
| `/api/v1/a2a/agents/:id/health-check` | POST | Health check individual |
| `/api/v1/a2a/agents/health-check-all` | POST | Health check de todos os agentes |
| `/api/v1/a2a/agents/:id/skills` | GET | Listar skills do agente |
| `/api/v1/a2a/agents/:id/skills` | POST | Vincular skill ao agente |
| `/api/v1/a2a/agents/:id/skills/:skillId` | DELETE | Desvincular skill |

---

## 6. Registro de Skills

Skills são capacidades reutilizáveis e combináveis que os agentes utilizam durante investigações.

### 6.1 Definição de Skill

| Campo | Tipo | Descrição |
|---|---|---|
| `name` | string | Nome legível por humanos |
| `slug` | string | Identificador único |
| `category` | enum | Categoria (ver seção 6.2) |
| `prompt_template` | string | Template base de prompt |
| `input_schema` | JSON Schema | Schema esperado de input |
| `output_schema` | JSON Schema | Schema esperado de output |
| `required_tools` | []string | Integrações de ferramentas necessárias |
| `resource_types` | []string | Tipos de recursos que a skill opera |
| `permissions` | []string | Permissões necessárias |

### 6.2 Categorias de Skills

| Categoria | Descrição |
|---|---|
| `infrastructure` | Operações de infraestrutura |
| `application` | Análise de aplicações |
| `database` | Operações de banco de dados |
| `network` | Diagnóstico de rede |
| `security` | Análise de segurança |
| `cloud` | Operações em nuvem |
| `observability` | Coleta e análise de observabilidade |
| `custom` | Skills customizadas |

### 6.3 Mapeamento Agente-Skill

Uma skill pode ser vinculada a múltiplos agentes. Cada agente pode sobrescrever a configuração da skill. O mapeamento é armazenado em `AgentSkill`.

### 6.4 Endpoints de Skills

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/skills` | GET | Listar todas as skills |
| `/api/v1/skills` | POST | Criar skill |
| `/api/v1/skills/:id` | GET | Detalhes da skill |
| `/api/v1/skills/:id` | PUT | Atualizar skill |
| `/api/v1/skills/:id` | DELETE | Remover skill |
| `/api/v1/skills/:id/agents` | GET | Listar agentes que usam a skill |

---

## 7. Credenciais e Acesso JIT

### 7.1 Provedores de Credencial

O sistema suporta backends de credencial plugáveis:

| Provider | Descrição |
|---|---|
| `hashicorp_vault` | HashiCorp Vault — segredos dinâmicos |
| `aws_sts` | AWS Security Token Service |
| `azure_managed_identity` | Azure Managed Identity |
| `gcp_workload_identity` | GCP Workload Identity Federation |
| `static` | Segredos estáticos (dev/fallback) |
| `custom` | Integração de provider customizado |

### 7.2 Tipos de Recursos

Credenciais são escopadas a recursos de software:

| Tipo de Recurso | Descrição |
|---|---|
| `kubernetes_cluster` | Acesso a cluster Kubernetes |
| `database` | Credenciais de banco de dados |
| `cloud_account` | Conta de provedor de nuvem |
| `api_endpoint` | Chave de API externa |
| `storage` | Object/block storage |
| `message_queue` | Acesso a sistema de filas |
| `cache` | Acesso a sistema de cache |
| `custom` | Recurso customizado |

### 7.3 Fluxo de Leasing JIT

```
Agente                Backend API              Credential Provider
   │                       │                          │
   │  POST /leases/request │                          │
   │──────────────────────►│                          │
   │                       │  valida AccessPolicy     │
   │                       │──────────────────────────┤
   │                       │  policy OK?              │
   │                       │◄─────────────────────────┤
   │                       │                          │
   │                       │  solicita credencial      │
   │                       │─────────────────────────►│
   │                       │  credencial com TTL       │
   │                       │◄─────────────────────────│
   │                       │                          │
   │                       │  registra lease (auditoria)
   │                       │                          │
   │  credencial + lease_id│                          │
   │◄──────────────────────│                          │
   │                       │                          │
   │  [usa credencial...]  │                          │
   │                       │                          │
   │  POST /leases/:id/revoke (opcional)              │
   │──────────────────────►│                          │
   │                       │  revoga na fonte         │
   │                       │─────────────────────────►│
```

**Steps do processo:**
1. Agente solicita lease via `POST /api/v1/credentials/leases/request`
2. Plataforma valida contra a Access Policy
3. Credencial é emitida com TTL configurado
4. Lease é registrado na trilha de auditoria
5. Lease pode ser revogado via `POST /api/v1/credentials/leases/:id/revoke`

### 7.4 Access Policies

Políticas de acesso controlam quais agentes/skills podem acessar quais recursos:

| Campo | Descrição |
|---|---|
| `target` | `agent` / `skill` / `agent_type` |
| `allowed_actions` | Lista de operações permitidas |
| `scope_restrictions` | Restrições de ambiente, região, etc. |
| `ttl` | Duração máxima do lease |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/access-policies` | GET | Listar políticas |
| `/api/v1/access-policies` | POST | Criar política |
| `/api/v1/access-policies/:id` | GET | Detalhes da política |
| `/api/v1/access-policies/:id` | PUT | Atualizar política |
| `/api/v1/access-policies/:id` | DELETE | Remover política |

### 7.5 Endpoints de Credenciais

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/credentials/providers` | GET | Listar providers |
| `/api/v1/credentials/providers` | POST | Criar provider |
| `/api/v1/credentials/providers/:id` | GET/PUT/DELETE | CRUD do provider |
| `/api/v1/software/:id/credentials` | GET | Listar credenciais do software |
| `/api/v1/software/:id/credentials` | POST | Criar credencial de recurso |
| `/api/v1/software/:id/credentials/:credId` | GET/PUT/DELETE | CRUD da credencial |
| `/api/v1/credentials/leases` | GET | Listar todos os leases |
| `/api/v1/credentials/leases/active` | GET | Listar leases ativos |
| `/api/v1/credentials/leases/request` | POST | Solicitar lease JIT |
| `/api/v1/credentials/leases/:id/revoke` | POST | Revogar lease |

---

## 8. Runbooks

### 8.1 Definição de Runbook

| Campo | Tipo | Descrição |
|---|---|---|
| `name` | string | Nome do runbook |
| `slug` | string | Identificador único |
| `description` | string | Propósito e uso |
| `trigger_conditions` | string | Quando utilizar este runbook |
| `auto_trigger` | bool | Se deve auto-executar em incidentes que correspondam |

### 8.2 Tipos de Steps

| Tipo | Descrição |
|---|---|
| `manual` | Humano deve completar este step |
| `automated` | Executado automaticamente por agente/skill |
| `approval` | Requer aprovação humana antes de prosseguir |
| `notification` | Envia uma notificação |
| `condition` | Lógica de branching |

Cada step suporta: timeout, estratégia de tratamento de falha, contagem de retries e vínculo com skill.

### 8.3 Execução de Runbook

Cada `RunbookExecution` rastreia:

| Campo | Descrição |
|---|---|
| `status` | Estado geral da execução |
| `current_step` | Step em andamento |
| `step_results` | Resultados por step |
| `triggered_by` | Quem ou o quê iniciou (humano ou auto) |
| Timing por step | Tempo gasto em cada step |

### 8.4 Fluxo de Execução

```
POST /api/v1/runbooks/:id/execute
          │
          ▼
  ┌───────────────┐
  │  Step 1       │  tipo: manual
  │  (aguarda)    │──── humano completa ───► POST /runbook-executions/:execId/steps/1/complete
  └───────┬───────┘
          │
          ▼
  ┌───────────────┐
  │  Step 2       │  tipo: automated
  │  (executa)    │──── skill executa automaticamente
  └───────┬───────┘
          │
          ▼
  ┌───────────────┐
  │  Step 3       │  tipo: approval
  │  (aguarda)    │──── aprovação humana necessária
  └───────┬───────┘
          │
          ▼
       [fim]
```

### 8.5 Endpoints de Runbooks

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/runbooks` | GET | Listar runbooks |
| `/api/v1/runbooks` | POST | Criar runbook |
| `/api/v1/runbooks/:id` | GET/PUT/DELETE | CRUD do runbook |
| `/api/v1/runbooks/:id/steps` | GET | Listar steps |
| `/api/v1/runbooks/:id/steps` | POST | Adicionar step |
| `/api/v1/runbooks/:id/steps/:stepId` | PUT/DELETE | CRUD do step |
| `/api/v1/runbooks/:id/execute` | POST | Iniciar execução |
| `/api/v1/runbook-executions/:execId` | GET | Status da execução |
| `/api/v1/runbook-executions/:execId` | PATCH | Atualizar execução |
| `/api/v1/runbook-executions/:execId/steps/:stepId/complete` | POST | Completar step manual |
| `/api/v1/incidents/:id/runbook-executions` | GET | Execuções de runbook do incidente |

---

## 9. Base de Conhecimento e Loop de Feedback

### 9.1 Knowledge Base

Conhecimento persistente e pesquisável construído a partir de incidentes resolvidos:

| Campo | Tipo | Descrição |
|---|---|---|
| `category` | string | Categoria do conhecimento |
| `error_pattern` | string | Padrão que dispara a busca |
| `root_cause_summary` | string | Causa raiz conhecida |
| `resolution` | string | Como resolver |
| `lessons_learned` | string | Aprendizados-chave |
| `action_items` | []string | Itens de ação padrão |
| `human_validated` | bool | Se foi validado por humano |
| `confidence` | float | Score de confiança |
| `reference_count` | int | Quantas vezes foi referenciado |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/knowledge-base` | GET | Listar entradas |
| `/api/v1/knowledge-base` | POST | Criar entrada |
| `/api/v1/knowledge-base/:id` | GET/PUT | CRUD da entrada |
| `/api/v1/knowledge-base/search` | POST | Busca semântica/keyword |

### 9.2 Feedback Humano

Após a análise, usuários podem enviar feedback sobre o output dos agentes:

| Campo | Tipo | Descrição |
|---|---|---|
| `target_type` | enum | `rci` / `rca` / `postmortem` / `triage` / `evidence` |
| `rating` | enum | `positive` / `negative` / `neutral` |
| `original_data` | JSON | O que o agente produziu |
| `corrected_data` | JSON | O que o humano corrigiu |

O feedback é usado para melhorar análises futuras e construir a base de conhecimento.

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/feedback` | GET | Listar feedback do incidente |
| `/api/v1/incidents/:id/feedback` | POST | Criar feedback |

### 9.3 Similar Incident Matching

- `GET /api/v1/incidents/:id/similar` — listar incidentes similares passados
- Score de similaridade + critérios de match armazenados por vínculo
- Usado nas fases de Evidence e RCA para referenciar padrões conhecidos

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/similar` | GET | Listar incidentes similares |
| `/api/v1/incidents/:id/similar` | POST | Criar vínculo de similaridade |

### 9.4 Correlation Rules

Regras que agrupam alertas relacionados em um único incidente:

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/correlation-rules` | GET | Listar regras |
| `/api/v1/correlation-rules` | POST | Criar regra |
| `/api/v1/correlation-rules/:id` | GET/PUT/DELETE | CRUD da regra |
| `/api/v1/incidents/:id/alert-groups` | GET | Ver alertas agrupados do incidente |

### 9.5 Endpoints

(Consolidados nas seções 9.1–9.4 acima.)

---

## 10. Fontes de Observabilidade

### 10.1 Fontes Suportadas

| Fonte | Descrição |
|---|---|
| Prometheus | Métricas e alertas Prometheus |
| Datadog | Métricas, logs e APM do Datadog |
| Grafana | Dashboards e alertas do Grafana |
| OpenTelemetry | Dados OTel (traces, metrics, logs) |
| Custom | Fonte customizada |

### 10.2 Snapshot Configs

Configurações de snapshot definem quais dados coletar automaticamente quando um incidente é disparado em um software:

| Tipo de dado | Descrição |
|---|---|
| Metric queries | Queries de métricas a executar |
| Log queries | Queries de log a executar |
| Dashboard screenshots | Screenshots de dashboards a capturar |
| Trace queries | Queries de trace a executar |

### 10.3 Endpoints de Observabilidade

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/observability/sources` | GET | Listar fontes |
| `/api/v1/observability/sources` | POST | Criar fonte |
| `/api/v1/observability/sources/:id` | GET/PUT/DELETE | CRUD da fonte |
| `/api/v1/observability/sources/:id/health` | POST | Verificar conectividade |
| `/api/v1/observability/sources/:id/snapshots` | GET | Listar snapshot configs |
| `/api/v1/observability/sources/:id/snapshots` | POST | Criar snapshot config |
| `/api/v1/observability/snapshots/:id` | GET/PUT/DELETE | CRUD do snapshot config |
| `/api/v1/software/:id/observability` | GET | Config de observabilidade do software |

---

## 11. Eventos de Mudança

Rastreia deployments e mudanças de infraestrutura para correlacionar com incidentes.

### 11.1 Modelo ChangeEvent

| Campo | Tipo | Descrição |
|---|---|---|
| `software_id` | uuid | Software afetado |
| `change_type` | enum | `deployment` / `config` / `infra` / `rollback` |
| `description` | string | O que mudou |
| `author` | string | Quem fez a mudança |
| `timestamp` | timestamp | Quando aconteceu |
| `metadata` | JSON | Contexto adicional (commit SHA, pipeline ID, etc.) |

### 11.2 Endpoints

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/change-events` | GET | Listar eventos de mudança |
| `/api/v1/change-events` | POST | Criar evento de mudança |
| `/api/v1/software/:id/changes` | GET | Listar mudanças de um software |

---

## 12. Notificações e Escalonamento

### 12.1 Canais de Notificação

| Tipo de Canal | Descrição |
|---|---|
| `slack` | Integração via webhook/bot do Slack |
| `teams` | Microsoft Teams |
| `pagerduty` | Alerta PagerDuty |
| `email` | Email via SMTP |
| `webhook` | HTTP webhook genérico |
| `custom` | Integração customizada |

### 12.2 Políticas de Escalonamento

| Campo | Descrição |
|---|---|
| Filtro de severidade | Escalar somente se `severity` ≥ threshold |
| Chains multi-step | Múltiplos passos de escalonamento em sequência |
| Repeat interval | Re-notificar se não houver reconhecimento |
| Canais vinculados | Canais de notificação associados |

### 12.3 Auditoria de Notificações

Cada notificação é registrada com:

| Campo | Descrição |
|---|---|
| Canal utilizado | Qual canal foi acionado |
| Política disparada | Qual política gerou a notificação |
| Tipo de evento | Qual evento causou a notificação |
| Destinatário | Quem recebeu |
| Status | `sent` / `failed` |
| Mensagem de erro | Se falhou |
| Timestamp de entrega | Quando foi entregue |

### 12.4 Endpoints

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/notification-channels` | GET | Listar canais |
| `/api/v1/notification-channels` | POST | Criar canal |
| `/api/v1/notification-channels/:id` | GET/PUT/DELETE | CRUD do canal |
| `/api/v1/escalation-policies` | GET | Listar políticas |
| `/api/v1/escalation-policies` | POST | Criar política |
| `/api/v1/escalation-policies/:id` | GET/PUT/DELETE | CRUD da política |
| `/api/v1/incidents/:id/notifications` | GET | Listar notificações do incidente |

---

## 13. Analytics

### 13.1 Métricas Disponíveis

| Endpoint | Método | Métrica |
|---|---|---|
| `/api/v1/analytics/mttr` | GET | Mean Time To Recovery |
| `/api/v1/analytics/trends` | GET | Tendências de volume de incidentes |
| `/api/v1/analytics/agent-effectiveness` | GET | Acurácia e performance dos agentes |
| `/api/v1/analytics/cost-by-model` | GET | Custo de tokens LLM por modelo |
| `/api/v1/analytics/cost-by-incident` | GET | Custo LLM por incidente |

### 13.2 Detalhamento de MTTR

O endpoint `/api/v1/analytics/mttr` decompõe o MTTR em:

| Métrica | Sigla | Descrição |
|---|---|---|
| Mean Time To Recovery | MTTR | Tempo total médio até recuperação |
| Time to Detect | TTD | Tempo entre início e detecção |
| Time to Acknowledge | TTA | Tempo para reconhecimento |
| Time to Mitigate | TTM | Tempo para mitigação |
| Time to Resolve | TTR | Tempo para resolução |

Suporta segmentação por: severidade, software, período de tempo.

### 13.3 Efetividade dos Agentes

O endpoint `/api/v1/analytics/agent-effectiveness` fornece:

| Métrica | Descrição |
|---|---|
| Acurácia de análise | Via ratings de feedback (positivo/negativo/neutro) |
| Distribuição de confidence score | Histograma de scores de confiança |
| Taxa de falsos positivos | Percentual de análises incorretas |
| Tempo economizado por investigação | Estimativa de tempo salvo |

### 13.4 Endpoints de Analytics

(Listados na seção 13.1 acima.)

---

## 14. Marketplace de Agentes

Catálogo de agentes pré-construídos que organizações podem instalar e configurar.

### 14.1 Modelo MarketplaceAgent

| Campo | Tipo | Descrição |
|---|---|---|
| `name` | string | Nome do agente |
| `slug` | string | Identificador único |
| `description` | string | O que o agente faz |
| `author` | string | Publicador |
| `version` | string | Versão semântica |
| `category` | string | Categoria do agente |
| `docker_image` | string | Container image |
| `agent_card` | JSON | A2A capability card |
| `skills` | []string | Skills incluídas |
| `required_credentials` | []string | Tipos de credencial necessários |
| `config_schema` | JSON Schema | Schema de configuração |
| `rating` | float | Avaliação da comunidade |
| `verified` | bool | Badge de verificação RootCauseway |
| `download_count` | int | Contagem de instalações |

A entidade `InstalledAgent` rastreia agentes instalados por organização, incluindo configuração, versão e status.

### 14.2 Endpoints do Marketplace

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/marketplace` | GET | Navegar catálogo |
| `/api/v1/marketplace/:slug` | GET | Detalhes do agente |
| `/api/v1/marketplace/:slug/install` | POST | Instalar agente |
| `/api/v1/marketplace/installed` | GET | Listar agentes instalados |
| `/api/v1/marketplace/installed/:id` | DELETE | Desinstalar agente |

---

## 15. Catálogo de Software

O catálogo de software é a fundação para a investigação contextualizada de incidentes.

### 15.1 Modelo SoftwareEntry

| Campo | Tipo | Descrição |
|---|---|---|
| `name` / `slug` | string | Identificador do serviço |
| `description` | string | O que o serviço faz |
| `owner` | string | Time ou pessoa responsável |
| `repository_url` | string | Repositório de código-fonte |
| `pipeline_url` | string | Pipeline de CI/CD |
| `cloud_provider` | enum | `aws` / `azure` / `gcp` / `on_prem` |
| `cloud_resources` | JSON | IDs/ARNs de recursos de nuvem |
| `database_info` | JSON | Detalhes do banco de dados |
| `infra_details` | JSON | Contexto de infraestrutura |
| `stakeholders` | []string | Contatos para incidentes |
| `sre_team` | string | Time SRE responsável |
| `architects` | []string | Arquitetos do sistema |
| `runbook_url` | string | Link para runbook |
| `dashboard_url` | string | Link para dashboard de monitoramento |
| `dependencies` | []string | Serviços upstream/downstream |

### 15.2 Uso do Catálogo na Investigação

O contexto completo de um `SoftwareEntry` é **injetado nos prompts de todos os agentes** durante cada investigação. Isso inclui: owner, SRE team, cloud provider, cloud resources, dependencies, stakeholders e infra_details.

### 15.3 Endpoints

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/software` | GET | Listar entradas |
| `/api/v1/software` | POST | Criar entrada |
| `/api/v1/software/:id` | GET/PUT/DELETE | CRUD da entrada |

---

## 16. Autenticação e Autorização

### 16.1 Métodos de Autenticação

| Método | Descrição |
|---|---|
| Local | Username + senha |
| SSO — OIDC | Generic OpenID Connect |
| SSO — Google | Google Workspace |
| SSO — GitHub | GitHub OAuth |
| SSO — Azure AD | Microsoft Entra ID |
| SSO — Okta | Okta OIDC |
| SSO — SAML | Generic SAML 2.0 |
| API Key | Acesso programático com escopo |

### 16.2 RBAC — Roles e Permissões

- Roles customizadas com permissões granulares
- Permissões são combinações de recurso + ação (ex: `incidents:write`, `agents:delete`)
- Usuários podem ter múltiplas roles
- Componente `PermissionGate` no frontend aplica RBAC na UI

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/roles` | GET | Listar roles |
| `/api/v1/roles` | POST | Criar role |
| `/api/v1/roles/:id` | GET/PUT/DELETE | CRUD da role |
| `/api/v1/roles/:id/permissions` | POST | Conceder permissão |
| `/api/v1/roles/:id/permissions/:permId` | DELETE | Revogar permissão |
| `/api/v1/users/:id/roles` | POST | Atribuir role ao usuário |
| `/api/v1/users/:id/roles/:roleId` | DELETE | Remover role do usuário |

### 16.3 API Keys

| Característica | Descrição |
|---|---|
| Escopo | Escopadas a recursos ou ações específicas |
| Revogação | Revogáveis a qualquer momento |
| Uso | Integrações e acesso via CLI |

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/auth/api-keys` | POST | Criar API key |
| `/api/v1/auth/api-keys` | GET | Listar API keys |
| `/api/v1/auth/api-keys/:id` | DELETE | Revogar API key |

### 16.4 Gerenciamento de Sessão

- Sessões baseadas em JWT
- Rastreamento de sessão com IP e user-agent
- Auto-provisionamento de usuários SSO

### 16.5 Fluxo de Autenticação SSO

```
Browser                 Backend API             SSO Provider
   │                        │                        │
   │  GET /auth/sso/:provider/login                  │
   │───────────────────────►│                        │
   │                        │ redirect               │
   │◄───────────────────────│                        │
   │                        │                        │
   │  [login no provider]───────────────────────────►│
   │                        │◄── authorization code ─│
   │                        │                        │
   │  GET /auth/sso/:provider/callback               │
   │───────────────────────►│                        │
   │                        │ troca code por token   │
   │                        │───────────────────────►│
   │                        │◄── id_token + profile ─│
   │                        │                        │
   │                        │ auto-provisionamento   │
   │                        │ se usuário não existe  │
   │                        │                        │
   │◄── JWT session ────────│                        │
```

### 16.6 Endpoints de Auth

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/auth/login` | POST | Login local |
| `/api/v1/auth/sso/:provider/login` | GET | Redirect para provider SSO |
| `/api/v1/auth/sso/:provider/callback` | GET | Callback do SSO |
| `/api/v1/auth/logout` | POST | Logout |
| `/api/v1/auth/me` | GET | Dados do usuário atual |
| `/api/v1/sso-providers` | GET | Listar providers SSO |
| `/api/v1/sso-providers` | POST | Criar provider SSO |
| `/api/v1/sso-providers/:id` | PUT/DELETE | CRUD do provider |

---

## 17. Comunicação em Tempo Real — WebSocket

### 17.1 Endpoint WebSocket

```
GET /ws
```

Eventos empurrados em tempo real para clientes conectados:

| Evento | Descrição |
|---|---|
| Mudança de status do incidente | Transições de estado |
| Progresso de agent run | Atualização de estágio do pipeline |
| Nova evidência adicionada | Artefato de evidência criado |
| Conclusão de estágio do pipeline | Cada fase do pipeline concluída |

### 17.2 Eventos Redis Pub/Sub

| Evento | Descrição |
|---|---|
| `alert.received` | Novo alerta ingerido |
| `triage.completed` | Fase de triage concluída |
| `evidence.collected` | Artefato de evidência adicionado |
| `hypothesis.generated` | Hipótese produzida |
| `agent.status` | Evento de ciclo de vida de agente |

### 17.3 Fluxo de Mensagens

```
Agente/Worker                   Redis                  Backend API           Browser
     │                            │                        │                    │
     │  publica evento            │                        │                    │
     │──────────────────────────►│                        │                    │
     │                            │  bridge (subscription) │                    │
     │                            │───────────────────────►│                    │
     │                            │                        │  push via WebSocket│
     │                            │                        │───────────────────►│
```

**Multi-instance safe:** O bridge Redis garante que todas as instâncias do backend consigam entregar mensagens WebSocket para qualquer cliente conectado, independente de qual instância está gerenciando a conexão.

---

## 18. Log de Auditoria

Toda ação que altera estado na plataforma é registrada automaticamente pelo `AuditMiddleware`.

### 18.1 Modelo AuditLog

| Campo | Tipo | Descrição |
|---|---|---|
| `actor_id` | uuid | Usuário ou sistema que executou a ação |
| `action` | string | Ação executada |
| `resource_type` | string | Tipo do recurso afetado |
| `resource_id` | uuid | ID do recurso específico |
| `changes` | JSON | Diff antes/depois |
| `ip_address` | string | IP de origem |
| `user_agent` | string | Informações do cliente |
| `timestamp` | timestamp | Quando aconteceu |

### 18.2 Endpoint

```
GET /api/v1/audit-log
```

Retorna trilha de auditoria consultável. O AuditMiddleware captura automaticamente todas as chamadas à API que alteram dados.

---

## 19. CLI

A CLI `rootcauseway` fornece acesso programático a todas as funcionalidades da plataforma.

### 19.1 Comandos Disponíveis

| Comando | Descrição |
|---|---|
| `rootcauseway config` | Gerenciar configuração da CLI |
| `rootcauseway auth` | Login, logout, gerenciar API keys |
| `rootcauseway agents` | Listar, criar, gerenciar agentes |
| `rootcauseway software` | Gerenciar catálogo de software |
| `rootcauseway incidents` | Listar e inspecionar incidentes |
| `rootcauseway analytics` | Consultar métricas de analytics |

Todos os comandos suportam a flag `--json` para output legível por máquina.

---

## 20. Frontend — Páginas e Componentes

### 20.1 Páginas

| Página | Rota | Descrição |
|---|---|---|
| Login | `/login` | Ponto de entrada de autenticação |
| Dashboard | `/` | Visão geral principal |
| Incidents | `/incidents` | Lista de incidentes com filtros |
| Incident Detail | `/incidents/:id` | Cockpit completo do incidente |
| Software | `/software` | Catálogo de serviços |
| Agents | `/agents` | Registro de agentes |
| Skills | `/skills` | Gerenciamento de skills |
| Marketplace | `/marketplace` | Marketplace de agentes |
| Webhooks | `/webhooks` | Configuração de webhooks |
| Credentials | `/credentials` | Gerenciamento de credenciais |
| Data Sources | `/data-sources` | Fontes de observabilidade |
| Runbooks | `/runbooks` | Biblioteca de runbooks |
| Analytics | `/analytics` | Métricas e dashboards |
| Notifications | `/notifications` | Configuração de notificações |
| Quarantine | `/quarantine` | Triage de alertas não associados |
| Users | `/users` | Gerenciamento de usuários |
| Roles | `/roles` | Configuração de RBAC |
| Audit Log | `/audit-log` | Trilha de auditoria |
| Settings | `/settings` | Configuração do sistema |
| Onboarding | `/onboarding` | Wizard de configuração inicial |

### 20.2 Componentes de UI

| Componente | Descrição |
|---|---|
| `RCAPanel` | Exibição dos resultados de Root Cause Analysis |
| `RCIPanel` | Painel de resumo da investigação |
| `PostmortemView` | Visualizador e editor de postmortem |
| `EvidencePanel` | Lista e detalhe de evidências |
| `EvidenceUpload` | Upload de arquivo de evidência manual |
| `IncidentTimeline` | Timeline cronológica de eventos |
| `RunsTimeline` | Visualização do DAG de execuções de agentes |
| `FiveWhys` | Visualizador do breakdown dos cinco porquês |
| `OrchestratorDecisions` | Log de decisões do orquestrador |
| `ConfidenceMeter` | Indicador visual de nível de confiança |
| `PresenceIndicator` | Presença de usuários em tempo real |
| `SeverityBadge` | Indicador de severidade com código de cor |
| `StatusBadge` | Indicador de estado do ciclo de vida |
| `PermissionGate` | Renderização condicional baseada em RBAC |
| `DataTable` | Grid de dados com ordenação e filtragem |

### 20.3 Hooks de Tempo Real

| Hook | Descrição |
|---|---|
| `useWebSocket` | Subscreve a atualizações em tempo real do incidente |
| `useAuth` | Contexto de sessão e permissões |
| `useToastMutation` | Notificações toast para mutations de API |

---

## 21. Referência Completa da API

### 21.1 Autenticação da API

Todos os endpoints (exceto `/api/v1/ingest/:token`) requerem autenticação via:
- Header `Authorization: Bearer <jwt>` para sessões de usuário
- Header `X-API-Key: <key>` para acesso programático

A especificação OpenAPI completa está disponível em:
```
GET /api/docs          → Swagger UI
GET /api/docs/openapi.yaml → Spec YAML
```

### 21.2 Todos os Endpoints por Domínio

#### Autenticação

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/auth/login` | POST | Login local |
| `/api/v1/auth/sso/:provider/login` | GET | Redirect SSO |
| `/api/v1/auth/sso/:provider/callback` | GET | Callback SSO |
| `/api/v1/auth/logout` | POST | Logout |
| `/api/v1/auth/me` | GET | Usuário atual |
| `/api/v1/auth/api-keys` | POST | Criar API key |
| `/api/v1/auth/api-keys` | GET | Listar API keys |
| `/api/v1/auth/api-keys/:id` | DELETE | Revogar API key |

#### Ingestão (Público)

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/ingest/:token` | POST | Ingestão de alerta externo |

#### Incidentes

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents` | GET | Listar incidentes |
| `/api/v1/incidents/:id` | GET | Detalhes |
| `/api/v1/incidents/:id` | PATCH | Atualizar |
| `/api/v1/incidents/:id/events` | POST | Adicionar evento |
| `/api/v1/incidents/:id/evidence` | POST | Adicionar evidência |
| `/api/v1/incidents/:id/evidence/upload` | POST | Upload de evidência |
| `/api/v1/incidents/:id/full` | GET | Cockpit completo |
| `/api/v1/incidents/:id/dag` | GET | DAG de investigação |
| `/api/v1/incidents/:id/runs` | GET | Agent runs |
| `/api/v1/incidents/:id/runs/:runId` | GET | Detalhes do run |
| `/api/v1/incidents/:id/runs/:runId/rerun` | POST | Reexecutar run |

#### Análise de Incidentes (RCI / RCA / Postmortem)

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/rci` | POST/GET/PATCH | RCI |
| `/api/v1/incidents/:id/rca` | POST/GET/PATCH | RCA |
| `/api/v1/incidents/:id/postmortem` | POST/GET/PATCH | Postmortem |
| `/api/v1/incidents/:id/orchestrator/decisions` | GET | Decisões do orquestrador |

#### A2A Agents

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/a2a/agents` | GET/POST | CRUD |
| `/api/v1/a2a/agents/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/a2a/agents/:id/card` | GET | Agent Card |
| `/api/v1/a2a/agents/:id/health-check` | POST | Health check |
| `/api/v1/a2a/agents/health-check-all` | POST | Health check todos |
| `/api/v1/a2a/agents/:id/skills` | GET/POST | Skills do agente |
| `/api/v1/a2a/agents/:id/skills/:skillId` | DELETE | Desvincular skill |

#### A2A Tasks

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/incidents/:id/a2a/tasks` | GET/POST | Tasks do incidente |
| `/api/v1/incidents/:id/a2a/tasks/:taskId` | GET/PATCH | Detalhes / atualização |

#### Skills

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/skills` | GET/POST | CRUD |
| `/api/v1/skills/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/skills/:id/agents` | GET | Agentes usando a skill |

#### Credenciais

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/credentials/providers` | GET/POST | CRUD providers |
| `/api/v1/credentials/providers/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/software/:id/credentials` | GET/POST | Credenciais do software |
| `/api/v1/software/:id/credentials/:credId` | GET/PUT/DELETE | CRUD |
| `/api/v1/credentials/leases` | GET | Listar leases |
| `/api/v1/credentials/leases/active` | GET | Leases ativos |
| `/api/v1/credentials/leases/request` | POST | Solicitar lease JIT |
| `/api/v1/credentials/leases/:id/revoke` | POST | Revogar lease |
| `/api/v1/access-policies` | GET/POST | CRUD políticas |
| `/api/v1/access-policies/:id` | GET/PUT/DELETE | CRUD |

#### Runbooks

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/runbooks` | GET/POST | CRUD |
| `/api/v1/runbooks/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/runbooks/:id/steps` | GET/POST | Steps do runbook |
| `/api/v1/runbooks/:id/steps/:stepId` | PUT/DELETE | CRUD |
| `/api/v1/runbooks/:id/execute` | POST | Iniciar execução |
| `/api/v1/runbook-executions/:execId` | GET/PATCH | Status / atualização |
| `/api/v1/runbook-executions/:execId/steps/:stepId/complete` | POST | Completar step |
| `/api/v1/incidents/:id/runbook-executions` | GET | Execuções do incidente |

#### Notificações e Escalonamento

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/notification-channels` | GET/POST | CRUD canais |
| `/api/v1/notification-channels/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/escalation-policies` | GET/POST | CRUD políticas |
| `/api/v1/escalation-policies/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/incidents/:id/notifications` | GET | Notificações do incidente |

#### Analytics

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/analytics/mttr` | GET | MTTR |
| `/api/v1/analytics/trends` | GET | Tendências |
| `/api/v1/analytics/agent-effectiveness` | GET | Efetividade de agentes |
| `/api/v1/analytics/cost-by-model` | GET | Custo por modelo LLM |
| `/api/v1/analytics/cost-by-incident` | GET | Custo por incidente |

#### Marketplace

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/marketplace` | GET | Navegar catálogo |
| `/api/v1/marketplace/installed` | GET | Instalados |
| `/api/v1/marketplace/:slug` | GET | Detalhes |
| `/api/v1/marketplace/:slug/install` | POST | Instalar |
| `/api/v1/marketplace/installed/:id` | DELETE | Desinstalar |

#### Catálogo de Software

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/software` | GET/POST | CRUD |
| `/api/v1/software/:id` | GET/PUT/DELETE | CRUD |

#### Usuários e Roles

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/users` | GET/POST | CRUD usuários |
| `/api/v1/users/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/users/:id/roles` | POST | Atribuir role |
| `/api/v1/users/:id/roles/:roleId` | DELETE | Remover role |
| `/api/v1/roles` | GET/POST | CRUD roles |
| `/api/v1/roles/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/roles/:id/permissions` | POST | Conceder permissão |
| `/api/v1/roles/:id/permissions/:permId` | DELETE | Revogar permissão |

#### SSO Providers

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/sso-providers` | GET/POST | CRUD providers |
| `/api/v1/sso-providers/:id` | PUT/DELETE | CRUD |

#### Webhooks

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/webhooks` | GET/POST | CRUD |
| `/api/v1/webhooks/:id` | GET/DELETE | CRUD |

#### Observabilidade

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/observability/sources` | GET/POST | CRUD fontes |
| `/api/v1/observability/sources/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/observability/sources/:id/health` | POST | Health check |
| `/api/v1/observability/sources/:id/snapshots` | GET/POST | Snapshot configs |
| `/api/v1/observability/snapshots/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/software/:id/observability` | GET | Observabilidade do software |

#### Knowledge Base e Feedback

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/knowledge-base` | GET/POST | CRUD |
| `/api/v1/knowledge-base/:id` | GET/PUT | CRUD |
| `/api/v1/knowledge-base/search` | POST | Busca |
| `/api/v1/incidents/:id/feedback` | GET/POST | Feedback do incidente |
| `/api/v1/incidents/:id/similar` | GET/POST | Incidentes similares |

#### Correlation Rules e Alert Groups

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/correlation-rules` | GET/POST | CRUD |
| `/api/v1/correlation-rules/:id` | GET/PUT/DELETE | CRUD |
| `/api/v1/incidents/:id/alert-groups` | GET | Grupos de alertas |

#### Change Events

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/change-events` | GET/POST | CRUD |
| `/api/v1/software/:id/changes` | GET | Mudanças do software |

#### Quarentena

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/quarantine` | GET | Listar |
| `/api/v1/quarantine/:id/resolve` | POST | Resolver |

#### Auditoria

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/audit-log` | GET | Trilha de auditoria |

#### Onboarding

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/v1/onboarding/status` | GET | Status do onboarding |

#### WebSocket

| Endpoint | Método | Descrição |
|---|---|---|
| `/ws` | GET (upgrade) | Canal de atualizações em tempo real |

#### Documentação

| Endpoint | Método | Descrição |
|---|---|---|
| `/api/docs` | GET | Swagger UI |
| `/api/docs/openapi.yaml` | GET | Spec OpenAPI |

### 21.3 Contagem por Domínio

| Domínio | Endpoints |
|---|---|
| Autenticação | 8 |
| Ingestão (público) | 1 |
| Incidentes (CRUD + eventos) | 12 |
| Análise de Incidentes (RCI/RCA/Postmortem) | 9 |
| Agent Runs & DAG | 4 |
| A2A Agents | 11 |
| A2A Tasks | 4 |
| Skills | 7 |
| Credenciais & Access Policies | 14 |
| Runbooks & Execuções | 14 |
| Notificações & Escalonamento | 11 |
| Analytics | 5 |
| Marketplace | 5 |
| Catálogo de Software | 5 |
| Usuários & Roles | 14 |
| SSO Providers | 4 |
| Webhooks | 4 |
| Observabilidade | 9 |
| Knowledge Base & Feedback | 7 |
| Correlation Rules & Alert Groups | 6 |
| Change Events | 3 |
| Quarentena | 2 |
| Auditoria | 1 |
| Onboarding | 1 |
| WebSocket | 1 |
| Documentação | 2 |
| **Total** | **~166** |

---

## 22. Infraestrutura e Deploy

### 22.1 Ambientes

#### Desenvolvimento

```bash
make up              # Inicia stack completa (docker compose)
make dev-backend     # Go API com hot reload
make dev-agent       # Orquestrador Python com hot reload
make dev-frontend    # Vite dev server
make down            # Para todos os serviços
```

#### Produção

```bash
cp .env.prod.example .env.prod   # Preencher valores reais
make prod-build                   # Build de todas as imagens
make prod-up                      # Iniciar stack de produção
make prod-logs                    # Tail de todos os logs
```

#### Banco de Dados

```bash
make db-up           # Iniciar postgres + redis
make db-migrate      # Aplicar migrações pendentes
make db-rollback     # Reverter última migração
make db-status       # Exibir estado das migrações
```

### 22.2 Variáveis de Ambiente

| Variável | Serviço | Descrição |
|---|---|---|
| `DB_HOST` | Backend | Host do PostgreSQL |
| `DB_PORT` | Backend | Porta do PostgreSQL |
| `DB_USER` | Backend | Usuário do banco |
| `DB_PASS` | Backend | Senha do banco |
| `DB_NAME` | Backend | Nome do banco |
| `REDIS_URL` | Backend/Orchestrator | URL de conexão Redis |
| `JWT_SECRET` | Backend | Chave de assinatura JWT |
| `ANTHROPIC_API_KEY` | Orchestrator/Agents | Chave de API LLM |
| `LOG_LEVEL` | Todos | Nível de logging |
| `PORT` | Cada serviço | Porta do serviço |
| `BACKEND_API_URL` | Orchestrator | URL da API Backend |

### 22.3 Histórico de Migrações do Banco de Dados

14 arquivos de migração rastreando a evolução completa do schema:

| Migration | Features Adicionadas |
|---|---|
| **001** | Schema core: organizations, users, software, incidents, webhooks, alert snapshots |
| **002** | Incident Cockpit: agent runs (DAG), RCI, RCA, Postmortem, blob storage de evidências |
| **003** | Protocolo A2A + catálogo de software enriquecido com contexto operacional, A2A agents, A2A tasks, decisões do orquestrador |
| **004** | Skills registry, mapeamento agente-skill, credential providers, resource credentials, access policies, credential leases |
| **005** | Feedback loop: feedback de incidentes, knowledge base com pattern matching, similar incidents |
| **006** | Correlation rules, alert grouping |
| **007** | Notification channels (Slack, Teams, PagerDuty, Email, Webhook), escalation policies, notification log |
| **008** | Runbooks, runbook steps, runbook execution tracking |
| **009** | Change events (deployments e mudanças de infraestrutura) |
| **010** | Auth completo: roles, permissões, SSO/OIDC, API keys, audit log, session management, auto-provisioning |
| **011** | Agent marketplace: catálogo de marketplace, installed agents por organização |
| **012** | Observability sources (Prometheus, Datadog, etc.), evidence snapshot configurations |
| **013** | Alert quarantine para webhooks não associados |
| **014** | Agent hosting model (Managed + BYOA) |

### 22.4 Middleware de Segurança da API

Toda requisição ao backend passa pela seguinte cadeia de middleware:

```
Requisição HTTP
      │
      ▼
┌─────────────┐
│  RequestID  │  injeta X-Request-ID único
└──────┬──────┘
       ▼
┌─────────────────┐
│ StructuredLogger│  logging JSON estruturado
└──────┬──────────┘
       ▼
┌─────────────────┐
│ SecurityHeaders │  CORS, CSP, headers de segurança
└──────┬──────────┘
       ▼
┌─────────────┐
│ RateLimiter │  100 req/min global, 20 req/min por usuário
└──────┬──────┘
       ▼
┌───────────────────────┐
│ UnifiedAuthMiddleware │  validação JWT + API key
└──────┬────────────────┘
       ▼
┌─────────────────┐
│ AuditMiddleware │  registro automático na trilha de auditoria
└──────┬──────────┘
       ▼
┌──────────┐
│ Recovery │  recuperação de panic
└──────┬───┘
       ▼
    Handler
```

### 22.5 Testes

```bash
make test            # Todos os testes
make test-backend    # Go tests (go test ./... -v -count=1)
make test-agent      # Python tests (pytest tests/ -v)
make test-frontend   # React tests (vitest run)
make lint            # golangci-lint + eslint
make ci              # Pipeline CI completo local (lint + test)
```

| Camada | Framework | Cobertura |
|---|---|---|
| Backend (Go) | go test | Handlers, services, repositories, integração |
| Agent Service (Python) | pytest | Crew factory, orchestration, integração E2E, models, snapshot collector, correlation engine |
| Frontend (React) | Vitest | Componentes, hooks |

### 22.6 Kubernetes

Configurações Terraform disponíveis em `/terraform/` para deploy em Kubernetes.

---

*Especificação Técnica RootCauseway — v1.0 | 2026-06-30*
*Baseado exclusivamente em `FEATURES.md`. Para adicionar seções, atualizar primeiro o inventário de features.*
