# Health Check

Check server health status.

**Endpoint:** `GET /api/v1/health`

**Request Body:** None

**Response 200 OK:**

```json
{
  "status": "success",
  "data": {
    "status": "ok",
    "service": "lonceng_unman_be",
    "version": "development"
  },
  "message": "Service is healthy"
}
```

| Field | JSON Path | Type | Example Value | Description |
|-------|-----------|------|---------------|-------------|
| Status | `status` | `string` | `"success"` | Always `"success"` for a successful health check |
| Message | `message` | `string` | `"Service is healthy"` | Human-readable status message |
| Data Status | `data.status` | `string` | `"ok"` | Always `"ok"` when the service is healthy |
| Service | `data.service` | `string` | `"lonceng_unman_be"` | Application name from `APP_NAME` env var |
| Version | `data.version` | `string` | `"development"` | Environment from `APP_ENV` env var |

**Example curl:**

```bash
curl http://localhost:3000/api/v1/health
```
