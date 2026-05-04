// Package protocol is the single source of truth for every protocol Spur supports.
// Adding a new protocol here automatically makes it available in:
//   - spur new  (asked during project creation)
//   - spur add protocol <name>  (added later)
//   - GEMINI.md generation  (agent awareness)
//   - docker-compose generation  (service config)
//   - .env.example generation  (required vars)
package protocol

// Protocol describes a runtime capability that the Spur platform can enable.
// It is not a module — it is infrastructure. Modules use protocols.
type Protocol struct {
	// ID is the short name used in CLI commands e.g. "grpc", "websocket"
	ID string

	// Label is shown in the interactive selector
	Label string

	// Description explains what this protocol enables
	Description string

	// WhatItEnables lists concrete things developers can build with it
	WhatItEnables []string

	// RequiresProtocols lists other protocols this one depends on
	RequiresProtocols []string

	// EnvVars are added to .env.example when this protocol is enabled
	EnvVars []EnvVar

	// DockerServices are appended to docker-compose.yml
	DockerServices []DockerService

	// InfraField is the field name in the Infra struct
	InfraField string

	// InfraType is the Go type of the field e.g. "*grpcserver.Server"
	InfraType string

	// BootstrapCode is added to bootstrap.go to initialise this protocol
	BootstrapCode string

	// ConfigFields are added to platform/config/config.go
	ConfigFields []ConfigField

	// AgentNote is written into GEMINI.md so the agent knows how to use it
	AgentNote string

	// AlwaysOn means this protocol cannot be removed (HTTP)
	AlwaysOn bool
}

type EnvVar struct {
	Key         string
	Default     string
	Description string
	Required    bool
}

type DockerService struct {
	Name    string
	Profile string // empty = always on, "temporal" = optional profile
	YAML    string
}

type ConfigField struct {
	Name    string
	Type    string
	EnvKey  string
	Default string
	Comment string
}

// ─── All Protocols ────────────────────────────────────────────────────────────

// All returns every protocol Spur knows about, in display order.
func All() []Protocol {
	return []Protocol{
		HTTP(),
		GRPC(),
		WebSocket(),
		SSE(),
		HLS(),
		RTMP(),
		Temporal(),
		Queue(),
	}
}

// Find returns a protocol by ID.
func Find(id string) (Protocol, bool) {
	for _, p := range All() {
		if p.ID == id {
			return p, true
		}
	}
	return Protocol{}, false
}

// ─── Protocol Definitions ─────────────────────────────────────────────────────

func HTTP() Protocol {
	return Protocol{
		ID:       "http",
		Label:    "HTTP / REST  (always on)",
		AlwaysOn: true,
		Description: "REST API server — always enabled. Every project gets this.",
		WhatItEnables: []string{
			"REST API endpoints",
			"OAuth2 / auth flows",
			"File upload / download",
			"Health check endpoint",
			"Admin panel API",
		},
		InfraField: "HTTP",
		InfraType:  "*httpserver.Server",
		EnvVars: []EnvVar{
			{Key: "HTTP_ADDR", Default: ":8080", Description: "HTTP server listen address"},
			{Key: "FRONTEND_URL", Default: "http://localhost:3000", Description: "Allowed CORS origin"},
			{Key: "RATE_LIMIT_ENABLED", Default: "true", Description: "Enable per-IP rate limiting"},
		},
		AgentNote: `## HTTP (always on)
Port: HTTP_ADDR (default :8080)
All REST handlers are registered via module.RegisterRoutes(r chi.Router).
CORS is pre-configured for FRONTEND_URL.
Rate limiting middleware is applied globally when RATE_LIMIT_ENABLED=true.
Health check: GET /healthz → {"status":"ok"}`,
	}
}

func GRPC() Protocol {
	return Protocol{
		ID:          "grpc",
		Label:       "gRPC — service-to-service, mobile SDKs, high-performance APIs",
		Description: "gRPC server with reflection. Best for mobile backends, internal services, high-perf APIs.",
		WhatItEnables: []string{
			"Mobile SDK backends (Flutter, Swift, Kotlin)",
			"Service-to-service calls",
			"Strongly typed APIs via protobuf",
			"Bi-directional streaming",
			"grpc-gateway (gRPC → REST bridge)",
		},
		InfraField: "GRPC",
		InfraType:  "*grpcserver.Server",
		ConfigFields: []ConfigField{
			{Name: "GRPCAddr", Type: "string", EnvKey: "GRPC_ADDR", Default: ":9090", Comment: "gRPC server address"},
			{Name: "GRPCReflection", Type: "bool", EnvKey: "GRPC_REFLECTION", Default: "true", Comment: "Enable gRPC reflection (grpcurl, Postman)"},
			{Name: "GRPCGateway", Type: "bool", EnvKey: "GRPC_GATEWAY", Default: "false", Comment: "Enable grpc-gateway (gRPC → REST)"},
		},
		EnvVars: []EnvVar{
			{Key: "GRPC_ADDR", Default: ":9090", Description: "gRPC server listen address"},
			{Key: "GRPC_REFLECTION", Default: "true", Description: "Enable gRPC server reflection"},
			{Key: "GRPC_GATEWAY", Default: "false", Description: "Enable gRPC-Gateway (REST bridge)"},
		},
		BootstrapCode: `// gRPC server
grpcSrv := grpcserver.New(cfg.GRPCAddr, cfg.GRPCReflection)`,
		AgentNote: `## gRPC
Port: GRPC_ADDR (default :9090)
Reflection enabled: test with grpcurl or Postman.
Register gRPC service in module.RegisterGRPC(infra.GRPC).
Proto files go in: proto/[module]/v1/[service].proto
Generate: make proto  (runs buf generate)
Test: grpcurl -plaintext localhost:9090 list`,
	}
}

func WebSocket() Protocol {
	return Protocol{
		ID:          "websocket",
		Label:       "WebSocket — real-time bidirectional communication",
		Description: "WebSocket hub with tenant-scoped channels. For live dashboards, chat, collaboration.",
		WhatItEnables: []string{
			"Live dashboards (monitoring, analytics)",
			"Chat and messaging",
			"Collaborative editing",
			"Real-time notifications",
			"Game state synchronisation",
			"Live proctoring events",
		},
		InfraField: "WS",
		InfraType:  "*wsserver.Hub",
		ConfigFields: []ConfigField{
			{Name: "WSPath", Type: "string", EnvKey: "WS_PATH", Default: "/ws", Comment: "WebSocket endpoint path"},
			{Name: "WSPingInterval", Type: "time.Duration", EnvKey: "WS_PING_INTERVAL", Default: "30s", Comment: "Keepalive ping interval"},
			{Name: "WSMaxMessageSize", Type: "int64", EnvKey: "WS_MAX_MSG_SIZE", Default: "65536", Comment: "Max message size in bytes"},
		},
		EnvVars: []EnvVar{
			{Key: "WS_PATH", Default: "/ws", Description: "WebSocket endpoint path"},
			{Key: "WS_PING_INTERVAL", Default: "30s", Description: "Keepalive ping interval"},
			{Key: "WS_MAX_MSG_SIZE", Default: "65536", Description: "Max WebSocket message size (bytes)"},
		},
		BootstrapCode: `// WebSocket hub
wsHub := wsserver.NewHub(cfg.WSPingInterval, cfg.WSMaxMessageSize)
go wsHub.Run()`,
		AgentNote: `## WebSocket
Endpoint: GET /ws?token=JWT  (JWT in query param, not header)
Hub is available via: infra.WS
Send to a tenant:     infra.WS.BroadcastTenant(tenantID, message)
Send to a user:       infra.WS.BroadcastUser(userID, message)
Send to a channel:    infra.WS.BroadcastChannel("jobs:uuid", message)
Subscribe a client:   infra.WS.Subscribe(conn, "jobs:uuid")
Message format: JSON  { "type": "event_type", "payload": {} }
Client JS:
  const ws = new WebSocket("ws://localhost:8080/ws?token=" + jwt)
  ws.onmessage = (e) => console.log(JSON.parse(e.data))`,
	}
}

func SSE() Protocol {
	return Protocol{
		ID:          "sse",
		Label:       "SSE (Server-Sent Events) — one-way server push, AI streaming",
		Description: "Server-Sent Events broker. Built into the HTTP server. For AI token streaming, live feeds, progress.",
		WhatItEnables: []string{
			"AI response streaming (token by token)",
			"Progress updates (uploads, exports, jobs)",
			"Live notification feeds",
			"Real-time log tailing",
			"Dashboard metric streams",
		},
		InfraField: "SSE",
		InfraType:  "*sseserver.Broker",
		ConfigFields: []ConfigField{
			{Name: "SSEKeepalive", Type: "time.Duration", EnvKey: "SSE_KEEPALIVE", Default: "15s", Comment: "SSE keepalive comment interval"},
		},
		EnvVars: []EnvVar{
			{Key: "SSE_KEEPALIVE", Default: "15s", Description: "SSE keepalive interval"},
		},
		BootstrapCode: `// SSE broker (built into HTTP, always available when SSE is enabled)
sseBroker := sseserver.NewBroker(cfg.SSEKeepalive)`,
		AgentNote: `## SSE (Server-Sent Events)
Broker available via: infra.SSE
Stream to a client:
  func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
      ch := infra.SSE.Subscribe(userID.String())
      defer infra.SSE.Unsubscribe(userID.String(), ch)
      sseserver.WriteHeaders(w)
      for msg := range ch {
          sseserver.WriteEvent(w, msg)
          w.(http.Flusher).Flush()
      }
  }
Send an event:    infra.SSE.Publish(userID.String(), sseserver.Event{Data: payload})
Client JS:
  const es = new EventSource("/stream/jobs?token=" + jwt)
  es.onmessage = (e) => console.log(JSON.parse(e.data))`,
	}
}

func HLS() Protocol {
	return Protocol{
		ID:          "hls",
		Label:       "HLS — HTTP Live Streaming (video delivery, VOD)",
		Description: "HLS segment server for video-on-demand and live stream delivery to any device.",
		WhatItEnables: []string{
			"Video on demand (courses, lectures, recordings)",
			"Live stream delivery to browsers and mobile",
			"Adaptive bitrate streaming",
			"Secure video with signed URLs",
		},
		RequiresProtocols: []string{"storage"},
		InfraField:        "HLS",
		InfraType:         "*hlsserver.Server",
		ConfigFields: []ConfigField{
			{Name: "HLSAddr", Type: "string", EnvKey: "HLS_ADDR", Default: ":8888", Comment: "HLS HTTP server address"},
			{Name: "HLSStoragePath", Type: "string", EnvKey: "HLS_STORAGE_PATH", Default: "./data/hls", Comment: "HLS segment storage directory"},
			{Name: "HLSSegmentDuration", Type: "int", EnvKey: "HLS_SEGMENT_DUR", Default: "4", Comment: "HLS segment duration in seconds"},
			{Name: "HLSCDNURL", Type: "string", EnvKey: "HLS_CDN_URL", Default: "", Comment: "Optional CDN URL prefix for HLS segments"},
		},
		EnvVars: []EnvVar{
			{Key: "HLS_ADDR", Default: ":8888", Description: "HLS segment server address"},
			{Key: "HLS_STORAGE_PATH", Default: "./data/hls", Description: "Directory for HLS segments"},
			{Key: "HLS_SEGMENT_DUR", Default: "4", Description: "Segment duration in seconds"},
			{Key: "HLS_CDN_URL", Default: "", Description: "CDN URL prefix (optional, leave empty for local)"},
		},
		BootstrapCode: `// HLS server
hlsSrv := hlsserver.New(cfg.HLSAddr, cfg.HLSStoragePath, cfg.HLSSegmentDuration)`,
		AgentNote: `## HLS (HTTP Live Streaming)
HLS server runs on: HLS_ADDR (default :8888)
Segments stored at: HLS_STORAGE_PATH (default ./data/hls)
Available via: infra.HLS

Create a stream:    infra.HLS.CreateStream(streamKey) → playbackURL
Delete a stream:    infra.HLS.DeleteStream(streamKey)
Playback URL:       http://host:8888/hls/{streamKey}/index.m3u8
Ingest (RTMP):      rtmp://host:1935/live/{streamKey}

Player (video.js):
  <video-js data-setup='{"sources": [{"src": "' + playbackURL + '", "type": "application/x-mpegURL"}]}'/>`,
	}
}

func RTMP() Protocol {
	return Protocol{
		ID:          "rtmp",
		Label:       "RTMP — live video ingest (OBS, mobile, ffmpeg)",
		Description: "RTMP server for live video ingest. Pairs with HLS for end-to-end live streaming.",
		WhatItEnables: []string{
			"Live video from OBS, Streamlabs, mobile apps",
			"Screen recording ingest",
			"Camera feed ingest (proctoring)",
			"Live events and webinars",
		},
		RequiresProtocols: []string{"hls"},
		InfraField:        "RTMP",
		InfraType:         "*rtmpserver.Server",
		ConfigFields: []ConfigField{
			{Name: "RTMPAddr", Type: "string", EnvKey: "RTMP_ADDR", Default: ":1935", Comment: "RTMP ingest server address"},
		},
		EnvVars: []EnvVar{
			{Key: "RTMP_ADDR", Default: ":1935", Description: "RTMP ingest address (standard port)"},
		},
		BootstrapCode: `// RTMP ingest server
rtmpSrv := rtmpserver.New(cfg.RTMPAddr, infra.HLS)`,
		AgentNote: `## RTMP (Live Video Ingest)
RTMP server on: RTMP_ADDR (default :1935)
Available via:  infra.RTMP

Stream key auth:   infra.RTMP.SetAuthHandler(func(streamKey string) bool { ... })
Get active streams: infra.RTMP.ActiveStreams() → []string
Ingest URL:        rtmp://host:1935/live/{streamKey}

OBS settings:
  Server: rtmp://your-server:1935/live
  Stream Key: {your-stream-key}

ffmpeg ingest:
  ffmpeg -i input.mp4 -c copy -f flv rtmp://localhost:1935/live/mykey`,
	}
}

func Temporal() Protocol {
	return Protocol{
		ID:          "temporal",
		Label:       "Temporal — durable workflows, long-running tasks, retries",
		Description: "Temporal workflow engine for durable execution. For AI agents, video processing, multi-step business flows.",
		WhatItEnables: []string{
			"Durable AI agent workflows (survives crashes)",
			"Long-running video transcoding",
			"Multi-step order processing",
			"Scheduled campaigns with complex logic",
			"Distributed transactions with compensation",
			"Background processing that must complete",
		},
		InfraField: "Temporal",
		InfraType:  "*temporalclient.Client",
		ConfigFields: []ConfigField{
			{Name: "TemporalHost", Type: "string", EnvKey: "TEMPORAL_HOST", Default: "", Comment: "Temporal server address (empty = disabled)"},
			{Name: "TemporalNamespace", Type: "string", EnvKey: "TEMPORAL_NAMESPACE", Default: "default", Comment: "Temporal namespace"},
			{Name: "TemporalWorker", Type: "bool", EnvKey: "TEMPORAL_WORKER", Default: "true", Comment: "Run Temporal worker in-process"},
		},
		EnvVars: []EnvVar{
			{Key: "TEMPORAL_HOST", Default: "localhost:7233", Description: "Temporal server address", Required: true},
			{Key: "TEMPORAL_NAMESPACE", Default: "default", Description: "Temporal namespace"},
			{Key: "TEMPORAL_WORKER", Default: "true", Description: "Run worker in-process (false = separate worker binary)"},
		},
		BootstrapCode: `// Temporal client (nil if TEMPORAL_HOST is empty)
var temporalClient *temporalclient.Client
if cfg.TemporalHost != "" {
    tc, err := temporalclient.Dial(temporalclient.Options{HostPort: cfg.TemporalHost})
    if err != nil {
        return nil, fmt.Errorf("temporal: %w", err)
    }
    temporalClient = &tc
}`,
		DockerServices: []DockerService{
			{
				Name:    "temporal",
				Profile: "temporal",
				YAML: `  temporal:
    image: temporalio/auto-setup:1.22
    profiles: ["temporal"]
    environment:
      DB: postgresql
      DB_PORT: 5432
      POSTGRES_USER: ${POSTGRES_USER:-spur}
      POSTGRES_PWD:  ${POSTGRES_PASSWORD:-changeme}
      POSTGRES_SEEDS: postgres
    ports:
      - "7233:7233"
    depends_on:
      postgres:
        condition: service_healthy`,
			},
			{
				Name:    "temporal-ui",
				Profile: "temporal",
				YAML: `  temporal-ui:
    image: temporalio/ui:2.21
    profiles: ["temporal"]
    environment:
      TEMPORAL_ADDRESS: temporal:7233
    ports:
      - "8081:8080"
    depends_on:
      - temporal`,
			},
		},
		AgentNote: `## Temporal
Client available via: infra.Temporal  (nil when TEMPORAL_HOST is not set)
Temporal UI: http://localhost:8081 (when started with --profile temporal)
Start with Temporal: docker compose --profile temporal up -d

Define a workflow:
  func MyWorkflow(ctx workflow.Context, input MyInput) (MyOutput, error) {
      ao := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute}
      ctx = workflow.WithActivityOptions(ctx, ao)
      var result MyOutput
      err := workflow.ExecuteActivity(ctx, MyActivity, input).Get(ctx, &result)
      return result, err
  }

Start a workflow:
  infra.Temporal.ExecuteWorkflow(ctx,
      client.StartWorkflowOptions{ID: "unique-id", TaskQueue: "mymodule-queue"},
      MyWorkflow, input)

Register in module Mount():
  if infra.Temporal != nil {
      w := worker.New(*infra.Temporal, "mymodule-queue", worker.Options{})
      w.RegisterWorkflow(MyWorkflow)
      w.RegisterActivity(MyActivity)
      go w.Run(worker.InterruptCh())
  }`,
	}
}

func Queue() Protocol {
	return Protocol{
		ID:          "queue",
		Label:       "Queue — async messaging via Redis Streams",
		Description: "Lightweight message queue using Redis Streams. No external queue service needed.",
		WhatItEnables: []string{
			"Async email/SMS delivery",
			"Background job dispatch",
			"Module-to-module event passing",
			"Webhook delivery queue",
			"Event sourcing patterns",
		},
		InfraField: "Queue",
		InfraType:  "*queue.Queue",
		ConfigFields: []ConfigField{
			{Name: "QueuePrefix", Type: "string", EnvKey: "QUEUE_PREFIX", Default: "spur", Comment: "Redis Streams key prefix"},
			{Name: "QueueMaxLen", Type: "int64", EnvKey: "QUEUE_MAX_LEN", Default: "10000", Comment: "Max messages per stream before trimming"},
		},
		EnvVars: []EnvVar{
			{Key: "QUEUE_PREFIX", Default: "spur", Description: "Redis Streams key prefix"},
			{Key: "QUEUE_MAX_LEN", Default: "10000", Description: "Max messages to retain per stream"},
		},
		BootstrapCode: `// Message queue (Redis Streams)
q := queue.New(redisClient, cfg.QueuePrefix, cfg.QueueMaxLen)`,
		AgentNote: `## Queue (Redis Streams)
Available via: infra.Queue
Backed by Redis Streams — no external queue service needed.

Publish a message:
  infra.Queue.Publish(ctx, "notifications:email", map[string]any{
      "to":      "user@example.com",
      "subject": "Welcome",
      "body":    "Hello!",
  })

Subscribe in module Mount():
  go infra.Queue.Subscribe(ctx, "notifications:email", func(msg queue.Message) error {
      // process msg.Data
      return nil
  })

Stream naming convention: "module:event"
Examples: "notifications:email", "webhooks:deliver", "jobs:execute"`,
	}
}
