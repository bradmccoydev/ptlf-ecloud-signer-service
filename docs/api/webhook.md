# Webhook API

## POST /webhook

Receives Harbor `SCANNING_COMPLETED` webhooks and triggers the image signing pipeline.

### Authentication

The webhook is authenticated via a shared secret in the `X-Harbor-Secret` HTTP header. The secret is compared using constant-time comparison to prevent timing attacks.

### Request

**Headers:**

| Header | Required | Description |
|--------|----------|-------------|
| `X-Harbor-Secret` | Yes | Shared secret from ExternalSecret `harbor-signer-webhook` |
| `Content-Type` | Yes | `application/json` |

**Body:**

```json
{
  "type": "SCANNING_COMPLETED",
  "occur_at": 1706000000,
  "operator": "system",
  "event_data": {
    "resources": [
      {
        "digest": "sha256:abc123def456...",
        "tag": "v1.2.3",
        "resource_url": "registry.platform.cuscal.io/platform/myapp:v1.2.3"
      }
    ],
    "repository": {
      "name": "platform/myapp",
      "namespace": "platform",
      "repo_full_name": "platform/myapp",
      "repo_type": "private"
    }
  }
}
```

### Responses

| Status | Condition |
|--------|-----------|
| **200** | Valid webhook accepted and signing job enqueued |
| **200** | Non-SCANNING_COMPLETED event discarded |
| **401** | Missing or invalid `X-Harbor-Secret` header |
| **400** | Payload exceeds 1MB or invalid JSON |
| **422** | Missing required fields (event type, digest, project, repo) |

### Example: Successful Acceptance

```bash
curl -X POST http://signer-service:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-Harbor-Secret: my-webhook-secret" \
  -d '{"type":"SCANNING_COMPLETED","event_data":{"resources":[{"digest":"sha256:abc123"}],"repository":{"name":"library/nginx","namespace":"library"}}}'
```

Response:
```json
{"status": "accepted"}
```

### Example: Invalid Secret

```bash
curl -X POST http://signer-service:8080/webhook \
  -H "X-Harbor-Secret: wrong-secret" \
  -d '{}'
```

Response (401):
```json
{"error": "unauthorized"}
```

### Behaviour Notes

- The handler returns 200 within 5 seconds — signing happens asynchronously in the worker pool
- Non-SCANNING_COMPLETED events (e.g., PUSH_ARTIFACT) are silently discarded with 200
- Duplicate webhooks for already-signed images are handled idempotently (no error, no re-signing)
