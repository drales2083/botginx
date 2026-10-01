---
title: Errors
description: API error responses and codes.
---

All errors return a consistent JSON structure:

```json
{
  "success": false,
  "error": {
    "code": "invalid_api_key",
    "message": "The provided API key is invalid or expired."
  }
}
```

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `missing_api_key` | 401 | No API key provided |
| `invalid_api_key` | 401 | Key is invalid or revoked |
| `forbidden` | 403 | Key lacks required scope |
| `not_found` | 404 | Resource does not exist |
| `rate_limit_exceeded` | 429 | Too many requests |
| `validation_error` | 422 | Invalid request data |
| `server_error` | 500 | Internal server error |

## Validation Errors

Validation errors include details about which fields failed:

```json
{
  "success": false,
  "error": {
    "code": "validation_error",
    "message": "Validation failed",
    "details": {
      "destinationUrls": "At least one destination URL is required",
      "domainId": "Invalid domain ID format"
    }
  }
}
```
