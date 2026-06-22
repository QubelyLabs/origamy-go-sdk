# Origamy SDK HTTP API Specification

## Overview

The HTTP service receives batched analytics events from the Origamy Go SDK (and other SDKs) at the `/v1/batch` endpoint. The wire format is intentionally identical to the Origamy Web SDK so a single backend handles all SDK types uniformly.

---

## Endpoint

```
POST /v1/batch
Host: events.origamy.io
```

Default base URL: `https://events.origamy.io`

---

## Authentication

**HTTP Basic Authentication**

- **Username**: `WriteKey` (the project/source write key)
- **Password**: Empty string (`""`)

```
Authorization: Basic base64(writeKey:)
```

---

## Request Headers

| Header           | Required | Value / Example                              |
| ---------------- | -------- | -------------------------------------------- |
| `Authorization`  | Yes      | `Basic <base64(writeKey:)>`                  |
| `Content-Type`   | Yes      | `application/json`                           |
| `Content-Length` | Yes      | Size of request body in bytes                |
| `User-Agent`     | Yes      | `origamy-go (version: 3.0.0)`                |

---

## Request Body Schema

```json
{
  "batch":  [ ...events ],
  "sentAt": "2024-01-15T10:30:00.123Z"
}
```

### Top-Level Fields

| Field     | Type             | Required | Description                                                       |
| --------- | ---------------- | -------- | ----------------------------------------------------------------- |
| `batch`   | array            | Yes      | Array of event messages (max 500KB total, max 32KB per message)   |
| `sentAt`  | string (ISO8601) | Yes      | Timestamp when the batch was dispatched, with milliseconds (`.sssZ`) |

> **Note**: Unlike some older analytics SDKs, there is no top-level `messageId` or `context` field. Context (including library info) is attached to each individual event in the `batch` array, matching the Origamy Web SDK format.

---

## Message Types

All messages in the `batch` array have a `type` field:

| Type       | Description                  |
| ---------- | ---------------------------- |
| `track`    | Track an action/event        |
| `identify` | Identify a user with traits  |
| `page`     | Track a page view (web)      |
| `screen`   | Track a screen view (mobile) |
| `group`    | Associate user with a group  |
| `alias`    | Merge two user identities    |

---

### Common Message Fields

| Field          | Type             | Required    | Description                                                           |
| -------------- | ---------------- | ----------- | --------------------------------------------------------------------- |
| `type`         | string           | Yes         | Message type                                                          |
| `messageId`    | string           | Yes         | Unique message identifier (UUID)                                      |
| `timestamp`    | string (ISO8601) | Yes         | When the event occurred (e.g. `2024-01-15T10:30:00Z`)                |
| `userId`       | string           | Conditional | User identifier — required unless `anonymousId` is provided           |
| `anonymousId`  | string           | Conditional | Anonymous user identifier — required unless `userId` is provided      |
| `context`      | object           | Yes         | Per-event context including library info (see Context Object Schema)  |
| `integrations` | object           | No          | Integration routing rules                                             |

---

### Track Message

```json
{
  "type": "track",
  "messageId": "uuid-001",
  "userId": "user-123",
  "event": "Order Completed",
  "timestamp": "2024-01-15T10:30:00Z",
  "properties": {
    "orderId": "ORD-9999",
    "revenue": 99.99,
    "currency": "USD",
    "products": [
      { "sku": "SKU-A", "name": "Widget", "price": 49.99, "quantity": 2 }
    ]
  },
  "context": {
    "library": { "name": "origamy-go", "version": "3.0.0" }
  }
}
```

**Required**: `event`, and either `userId` or `anonymousId`

---

### Identify Message

```json
{
  "type": "identify",
  "messageId": "uuid-002",
  "userId": "user-123",
  "timestamp": "2024-01-15T10:30:00Z",
  "traits": {
    "email": "user@example.com",
    "firstName": "Alice",
    "lastName": "Smith",
    "phone": "+15551234567",
    "plan": "enterprise",
    "createdAt": "2023-01-01T00:00:00Z"
  },
  "context": {
    "library": { "name": "origamy-go", "version": "3.0.0" }
  }
}
```

**Required**: Either `userId` or `anonymousId`

---

### Page Message

```json
{
  "type": "page",
  "messageId": "uuid-003",
  "userId": "user-123",
  "name": "Pricing",
  "timestamp": "2024-01-15T10:30:00Z",
  "properties": {
    "url":      "https://example.com/pricing",
    "path":     "/pricing",
    "title":    "Pricing Plans",
    "referrer": "https://google.com"
  },
  "context": {
    "library":   { "name": "origamy-go", "version": "3.0.0" },
    "userAgent": "Go-http-client/2.0",
    "locale":    "en-US"
  }
}
```

**Required**: Either `userId` or `anonymousId`

---

### Screen Message

```json
{
  "type": "screen",
  "messageId": "uuid-004",
  "userId": "mobile-user-7",
  "name": "Dashboard",
  "timestamp": "2024-01-15T10:30:00Z",
  "properties": {
    "category": "Main",
    "tab": "overview"
  },
  "context": {
    "library": { "name": "origamy-go", "version": "3.0.0" },
    "os":     { "name": "iOS", "version": "17.2" },
    "device": { "manufacturer": "Apple", "model": "iPhone 15", "type": "mobile" },
    "screen": { "width": 390, "height": 844, "density": 3 }
  }
}
```

**Required**: Either `userId` or `anonymousId`

---

### Group Message

```json
{
  "type": "group",
  "messageId": "uuid-005",
  "userId": "user-123",
  "groupId": "company-acme",
  "timestamp": "2024-01-15T10:30:00Z",
  "traits": {
    "name":    "Acme Corp",
    "website": "https://acme.com",
    "plan":    "enterprise",
    "mrr":     12000
  },
  "context": {
    "library": { "name": "origamy-go", "version": "3.0.0" }
  }
}
```

**Required**: `groupId`, and either `userId` or `anonymousId`

---

### Alias Message

```json
{
  "type": "alias",
  "messageId": "uuid-006",
  "userId": "identified-user-1",
  "previousId": "anon-session-xyz",
  "timestamp": "2024-01-15T10:30:00Z",
  "context": {
    "library": { "name": "origamy-go", "version": "3.0.0" }
  }
}
```

**Required**: `userId`, `previousId`

---

## Context Object Schema

Context is attached to each event individually. The SDK always populates `library` with the SDK name and version. Applications can add or override context fields per-event:

```json
{
  "library": {
    "name":    "origamy-go",
    "version": "3.0.0"
  },
  "app": {
    "name":      "MyApp",
    "version":   "2.0.0",
    "build":     "123",
    "namespace": "com.example.myapp"
  },
  "campaign": {
    "name":    "Winter Sale",
    "source":  "email",
    "medium":  "newsletter",
    "term":    "winter deals",
    "content": "ad-variation-1"
  },
  "device": {
    "id":           "device-uuid",
    "manufacturer": "Apple",
    "model":        "iPhone 15",
    "name":         "Alice's iPhone",
    "type":         "mobile",
    "version":      "17.0",
    "advertisingId":"ad-id"
  },
  "location": {
    "city":      "San Francisco",
    "country":   "USA",
    "region":    "California",
    "latitude":  37.7749,
    "longitude": -122.4194
  },
  "network": {
    "bluetooth": false,
    "cellular":  true,
    "wifi":      false,
    "carrier":   "Verizon"
  },
  "os": {
    "name":    "iOS",
    "version": "17.0"
  },
  "page": {
    "path":     "/products",
    "referrer": "https://google.com",
    "search":   "?q=search",
    "title":    "Products",
    "url":      "https://example.com/products"
  },
  "screen": {
    "density": 2,
    "width":   1920,
    "height":  1080
  },
  "ip":        "192.168.1.1",
  "locale":    "en-US",
  "timezone":  "America/Los_Angeles",
  "userAgent": "Go-http-client/2.0"
}
```

---

## Integrations Object

Controls which downstream integrations receive the event:

```json
{
  "All":       true,
  "Mixpanel":  true,
  "Salesforce": false
}
```

---

## Response Codes

| Status                      | Description                      |
| --------------------------- | -------------------------------- |
| `200 OK`                    | Batch accepted successfully      |
| `400 Bad Request`           | Invalid JSON or validation error |
| `401 Unauthorized`          | Invalid or missing write key     |
| `413 Payload Too Large`     | Batch exceeds 500KB limit        |
| `429 Too Many Requests`     | Rate limit exceeded              |
| `500 Internal Server Error` | Server error                     |

### Success Response

```json
{ "success": true }
```

### Error Response

```json
{ "success": false, "error": "Error description" }
```

---

## Limits

| Limit                  | Value                 |
| ---------------------- | --------------------- |
| Max batch size         | 500,000 bytes (500KB) |
| Max message size       | 32,000 bytes (32KB)   |
| Request timeout        | 10 seconds            |
| Retry attempts         | 10 (exponential backoff) |

---

## Full Example Request

```http
POST /v1/batch HTTP/1.1
Host: events.origamy.io
Authorization: Basic d3JpdGVfa2V5Xzk4NzY1NDMyMTA6
Content-Type: application/json
Content-Length: 987
User-Agent: origamy-go (version: 3.0.0)

{
  "batch": [
    {
      "type": "identify",
      "messageId": "msg-001",
      "userId": "user-123",
      "timestamp": "2024-01-15T10:30:00Z",
      "traits": {
        "email": "alice@example.com",
        "name":  "Alice Smith",
        "plan":  "enterprise"
      },
      "context": {
        "library": { "name": "origamy-go", "version": "3.0.0" }
      }
    },
    {
      "type": "track",
      "messageId": "msg-002",
      "userId": "user-123",
      "event": "Order Completed",
      "timestamp": "2024-01-15T10:30:01Z",
      "properties": {
        "orderId":  "ORD-9999",
        "revenue":  99.99,
        "currency": "USD"
      },
      "context": {
        "library": { "name": "origamy-go", "version": "3.0.0" }
      }
    }
  ],
  "sentAt": "2024-01-15T10:30:01.234Z"
}
```

---

## Validation Rules

| Message Type | Required Fields                                             |
| ------------ | ----------------------------------------------------------- |
| `track`      | `event`, (`userId` OR `anonymousId`)                        |
| `identify`   | (`userId` OR `anonymousId`)                                 |
| `page`       | (`userId` OR `anonymousId`)                                 |
| `screen`     | (`userId` OR `anonymousId`)                                 |
| `group`      | `groupId`, (`userId` OR `anonymousId`)                      |
| `alias`      | `userId`, `previousId`                                      |

`messageId` and `timestamp` are always auto-populated by the SDK when not provided by the caller.
