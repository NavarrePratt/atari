# Notifications

Webhook-based notification system for external alerts on key events.

## Purpose

The Notifications component is responsible for:
- Sending HTTP webhooks on configurable events
- Supporting multiple notification providers (IFTTT, Slack, Discord, generic webhooks)
- Providing retry logic for failed notifications
- Batching notifications to avoid spam

## Interface

```go
type Notifier struct {
    config   *config.NotificationConfig
    client   *http.Client
    events   <-chan events.Event
    queue    chan notification
}

type notification struct {
    Provider string
    Event    events.Event
    Payload  map[string]any
}

// Public API
func New(cfg *config.NotificationConfig) *Notifier
func (n *Notifier) Start(ctx context.Context, events <-chan events.Event) error
func (n *Notifier) Stop() error
```

## Dependencies

| Component | Usage |
|-----------|-------|
| config.Config | Provider settings, triggers |
| events.Router | Subscribe to event stream |

## Supported Providers

### IFTTT Webhooks

```yaml
notifications:
  ifttt:
    enabled: true
    key: "your-ifttt-webhook-key"
    event_name: "atari_notification"
    triggers:
      - iteration.end
      - error
      - drain.stop
```

IFTTT webhook format:
```
POST https://maker.ifttt.com/trigger/{event}/with/key/{key}
Content-Type: application/json

{
  "value1": "Bead completed",
  "value2": "bd-042: Fix auth bug",
  "value3": "$0.42"
}
```

### Slack Incoming Webhooks

```yaml
notifications:
  slack:
    enabled: true
    webhook_url: "https://hooks.slack.com/services/XXX/YYY/ZZZ"
    channel: "#atari-notifications"
    triggers:
      - iteration.end
      - error
```

Slack message format:
```json
{
  "channel": "#atari-notifications",
  "username": "Atari",
  "icon_emoji": ":robot_face:",
  "attachments": [
    {
      "color": "good",
      "title": "Bead Completed",
      "text": "bd-042: Fix auth bug",
      "fields": [
        {"title": "Cost", "value": "$0.42", "short": true},
        {"title": "Turns", "value": "8", "short": true}
      ]
    }
  ]
}
```

### Discord Webhooks

```yaml
notifications:
  discord:
    enabled: true
    webhook_url: "https://discord.com/api/webhooks/XXX/YYY"
    triggers:
      - iteration.end
      - error
```

Discord embed format:
```json
{
  "username": "Atari",
  "embeds": [
    {
      "title": "Bead Completed",
      "description": "bd-042: Fix auth bug",
      "color": 5763719,
      "fields": [
        {"name": "Cost", "value": "$0.42", "inline": true},
        {"name": "Turns", "value": "8", "inline": true}
      ]
    }
  ]
}
```

### Generic Webhook

```yaml
notifications:
  webhook:
    enabled: true
    url: "https://your-server.com/webhook"
    method: POST
    headers:
      Authorization: "Bearer your-token"
      Content-Type: "application/json"
    triggers:
      - iteration.end
      - error
      - drain.start
      - drain.stop
```

Generic payload:
```json
{
  "event": "iteration.end",
  "timestamp": "2024-01-15T14:05:00Z",
  "data": {
    "bead_id": "bd-042",
    "title": "Fix auth bug",
    "success": true,
    "cost_usd": 0.42,
    "turns": 8,
    "duration_ms": 115000
  }
}
```

## Implementation

### Configuration

```go
type NotificationConfig struct {
    IFTTT   *IFTTTConfig   `yaml:"ifttt"`
    Slack   *SlackConfig   `yaml:"slack"`
    Discord *DiscordConfig `yaml:"discord"`
    Webhook *WebhookConfig `yaml:"webhook"`
}

type IFTTTConfig struct {
    Enabled   bool     `yaml:"enabled"`
    Key       string   `yaml:"key"`
    EventName string   `yaml:"event_name"`
    Triggers  []string `yaml:"triggers"`
}

type SlackConfig struct {
    Enabled    bool     `yaml:"enabled"`
    WebhookURL string   `yaml:"webhook_url"`
    Channel    string   `yaml:"channel"`
    Triggers   []string `yaml:"triggers"`
}

type DiscordConfig struct {
    Enabled    bool     `yaml:"enabled"`
    WebhookURL string   `yaml:"webhook_url"`
    Triggers   []string `yaml:"triggers"`
}

type WebhookConfig struct {
    Enabled  bool              `yaml:"enabled"`
    URL      string            `yaml:"url"`
    Method   string            `yaml:"method"`
    Headers  map[string]string `yaml:"headers"`
    Triggers []string          `yaml:"triggers"`
}
```

### Event Filtering

```go
func (n *Notifier) Start(ctx context.Context, events <-chan events.Event) error {
    n.queue = make(chan notification, 100)

    go n.processQueue(ctx)
    go n.filterEvents(ctx, events)

    return nil
}

func (n *Notifier) filterEvents(ctx context.Context, events <-chan events.Event) {
    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-events:
            if !ok {
                return
            }
            n.maybeNotify(event)
        }
    }
}

func (n *Notifier) maybeNotify(event events.Event) {
    eventType := string(event.Type())

    // Check each provider
    if n.config.IFTTT != nil && n.config.IFTTT.Enabled {
        if contains(n.config.IFTTT.Triggers, eventType) {
            n.queue <- notification{
                Provider: "ifttt",
                Event:    event,
            }
        }
    }

    if n.config.Slack != nil && n.config.Slack.Enabled {
        if contains(n.config.Slack.Triggers, eventType) {
            n.queue <- notification{
                Provider: "slack",
                Event:    event,
            }
        }
    }

    // ... similar for discord, webhook
}
```

### Sending Notifications

```go
func (n *Notifier) processQueue(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case notif := <-n.queue:
            n.send(ctx, notif)
        }
    }
}

func (n *Notifier) send(ctx context.Context, notif notification) {
    var err error

    switch notif.Provider {
    case "ifttt":
        err = n.sendIFTTT(ctx, notif.Event)
    case "slack":
        err = n.sendSlack(ctx, notif.Event)
    case "discord":
        err = n.sendDiscord(ctx, notif.Event)
    case "webhook":
        err = n.sendWebhook(ctx, notif.Event)
    }

    if err != nil {
        // Log but don't fail - notifications are best-effort
        fmt.Fprintf(os.Stderr, "notification error (%s): %v\n", notif.Provider, err)
    }
}
```

### IFTTT Implementation

```go
func (n *Notifier) sendIFTTT(ctx context.Context, event events.Event) error {
    cfg := n.config.IFTTT
    url := fmt.Sprintf("https://maker.ifttt.com/trigger/%s/with/key/%s",
        cfg.EventName, cfg.Key)

    value1, value2, value3 := n.formatIFTTT(event)

    payload := map[string]string{
        "value1": value1,
        "value2": value2,
        "value3": value3,
    }

    return n.post(ctx, url, payload, nil)
}

func (n *Notifier) formatIFTTT(event events.Event) (string, string, string) {
    switch e := event.(type) {
    case *events.IterationEnd:
        status := "completed"
        if !e.Success {
            status = "failed"
        }
        return fmt.Sprintf("Bead %s", status),
            e.BeadID,
            fmt.Sprintf("$%.2f | %d turns", e.TotalCostUSD, e.NumTurns)

    case *events.DrainStart:
        return "Atari started", e.WorkDir, ""

    case *events.DrainStop:
        return "Atari stopped", e.Reason, ""

    case *events.Error:
        return "Error", e.Message, e.BeadID

    default:
        return string(event.Type()), "", ""
    }
}
```

### Slack Implementation

```go
func (n *Notifier) sendSlack(ctx context.Context, event events.Event) error {
    cfg := n.config.Slack

    payload := n.formatSlack(event, cfg.Channel)
    return n.post(ctx, cfg.WebhookURL, payload, nil)
}

func (n *Notifier) formatSlack(event events.Event, channel string) map[string]any {
    attachment := n.slackAttachment(event)

    return map[string]any{
        "channel":    channel,
        "username":   "Atari",
        "icon_emoji": ":robot_face:",
        "attachments": []map[string]any{attachment},
    }
}

func (n *Notifier) slackAttachment(event events.Event) map[string]any {
    switch e := event.(type) {
    case *events.IterationEnd:
        color := "good"
        title := "Bead Completed"
        if !e.Success {
            color = "danger"
            title = "Bead Failed"
        }
        return map[string]any{
            "color": color,
            "title": title,
            "text":  e.BeadID,
            "fields": []map[string]any{
                {"title": "Cost", "value": fmt.Sprintf("$%.2f", e.TotalCostUSD), "short": true},
                {"title": "Turns", "value": fmt.Sprintf("%d", e.NumTurns), "short": true},
            },
        }

    case *events.Error:
        return map[string]any{
            "color": "danger",
            "title": "Error",
            "text":  e.Message,
        }

    default:
        return map[string]any{
            "title": string(event.Type()),
        }
    }
}
```

### HTTP Helper

```go
func (n *Notifier) post(ctx context.Context, url string, payload any, headers map[string]string) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    for k, v := range headers {
        req.Header.Set(k, v)
    }

    resp, err := n.client.Do(req)
    if err != nil {
        return fmt.Errorf("send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(body))
    }

    return nil
}
```

## Trigger Events

Recommended triggers by use case:

| Use Case | Triggers |
|----------|----------|
| Progress updates | `iteration.end` |
| Error alerts | `error` |
| Abandoned beads | `bead.abandoned` |
| Session boundaries | `drain.start`, `drain.stop` |
| Cost tracking | `session.end` |
| Overnight monitoring | `error`, `bead.abandoned`, `drain.stop` |
| All activity | `*` (not recommended - too noisy) |

### bead.abandoned Event

Triggered when a bead hits the `max_failures` limit and is abandoned. This is critical for overnight runs - you want to know if work is getting stuck rather than silently burning tokens.

Payload:
```json
{
  "event": "bead.abandoned",
  "timestamp": "2024-01-15T03:42:00Z",
  "data": {
    "bead_id": "bd-042",
    "attempts": 5,
    "max_failures": 5,
    "last_error": "Tests failed: 3 assertions"
  }
}
```

Example Slack notification for abandoned beads:
```go
case *events.BeadAbandoned:
    return map[string]any{
        "color": "warning",
        "title": "Bead Abandoned",
        "text":  fmt.Sprintf("%s abandoned after %d attempts", e.BeadID, e.Attempts),
        "fields": []map[string]any{
            {"title": "Last Error", "value": e.LastError, "short": false},
        },
    }
```

## Rate Limiting

To avoid notification spam:

```go
type rateLimiter struct {
    lastSent map[string]time.Time
    minDelay time.Duration
    mu       sync.Mutex
}

func (r *rateLimiter) Allow(key string) bool {
    r.mu.Lock()
    defer r.mu.Unlock()

    last, exists := r.lastSent[key]
    if exists && time.Since(last) < r.minDelay {
        return false
    }

    r.lastSent[key] = time.Now()
    return true
}
```

Configuration:
```yaml
notifications:
  rate_limit:
    min_delay: 30s  # Minimum time between notifications of same type
    batch_window: 5s  # Batch events within this window
```

## Retry Logic

```go
func (n *Notifier) sendWithRetry(ctx context.Context, notif notification) error {
    var lastErr error

    for attempt := 0; attempt < 3; attempt++ {
        if attempt > 0 {
            time.Sleep(time.Duration(attempt) * 5 * time.Second)
        }

        err := n.send(ctx, notif)
        if err == nil {
            return nil
        }
        lastErr = err
    }

    return fmt.Errorf("notification failed after 3 attempts: %w", lastErr)
}
```

## Testing

### Unit Tests

- Payload formatting for each provider
- Trigger filtering logic
- Rate limiting

### Integration Tests

- Actual webhook delivery (use webhook.site or similar)
- Error handling for failed webhooks

### Test Fixtures

```go
func TestIFTTTPayload(t *testing.T) {
    n := &Notifier{config: &config.NotificationConfig{}}

    event := &events.IterationEnd{
        BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
        BeadID:       "bd-042",
        Success:      true,
        TotalCostUSD: 0.42,
        NumTurns:     8,
    }

    v1, v2, v3 := n.formatIFTTT(event)

    if v1 != "Bead completed" {
        t.Errorf("expected 'Bead completed', got %q", v1)
    }
    if v2 != "bd-042" {
        t.Errorf("expected 'bd-042', got %q", v2)
    }
}
```

## Error Handling

| Error | Action |
|-------|--------|
| Webhook timeout | Retry with backoff |
| 4xx response | Log and skip (bad config) |
| 5xx response | Retry |
| Network error | Retry with backoff |

## Security Considerations

- Store webhook URLs/keys in config file with restricted permissions
- Use HTTPS for all webhooks
- Consider secrets management for production
- Sanitize event data before sending (no sensitive paths, etc.)

## Future Considerations

- **Email notifications**: SMTP integration
- **Pushover**: Mobile push notifications
- **Custom templates**: User-defined message formats
- **Notification history**: Track sent notifications
- **Digest mode**: Send daily/hourly summaries instead of per-event
