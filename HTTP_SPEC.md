# Origamy SDK HTTP API Specification

## Overview

The HTTP service receives batched analytics events from the Origamy Go SDK (and other SDKs) at the `/v1/batch` endpoint.

---

## Endpoint

```
POST /v1/batch
```

---

## Authentication

**HTTP Basic Authentication**

- **Username**: `WriteKey` (the project/source write key)
- **Password**: Empty string (`""`)

The write key is sent in the `Authorization` header using standard Basic Auth encoding:

```
Authorization: Basic base64(writeKey:)
```

---

## Request Headers

| Header           | Required | Description                                         |
| ---------------- | -------- | --------------------------------------------------- |
| `Authorization`  | Yes      | Basic auth with write key as username               |
| `Content-Type`   | Yes      | `application/json`                                  |
| `Content-Length` | Yes      | Size of request body in bytes                       |
| `User-Agent`     | Yes      | SDK identifier, e.g., `origamy-go (version: 3.0.0)` |

---

## Request Body Schema

```json
{
  "messageId": "string",
  "sentAt": "ISO8601 timestamp",
  "context": { ... },
  "batch": [ ... ]
}
```

### Top-Level Fields

| Field       | Type             | Required | Description                                                     |
| ----------- | ---------------- | -------- | --------------------------------------------------------------- |
| `messageId` | string           | Yes      | Unique batch identifier (UUID)                                  |
| `sentAt`    | string (ISO8601) | Yes      | Timestamp when batch was sent                                   |
| `context`   | object           | Yes      | SDK/library context                                             |
| `batch`     | array            | Yes      | Array of event messages (max 500KB total, max 32KB per message) |

### Batch Context Object

```json
{
  "library": {
    "name": "origamy-go",
    "version": "3.0.0"
  }
}
```

---

## Message Types

All messages in the `batch` array have a `type` field. Supported types:

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
| `type`         | string           | Yes         | Message type: `track`, `identify`, `page`, `screen`, `group`, `alias` |
| `messageId`    | string           | Yes         | Unique message identifier                                             |
| `timestamp`    | string (ISO8601) | Yes         | When the event occurred                                               |
| `userId`       | string           | Conditional | User identifier (required unless `anonymousId` provided)              |
| `anonymousId`  | string           | Conditional | Anonymous user identifier                                             |
| `context`      | object           | No          | Event-level context (merges with batch context)                       |
| `integrations` | object           | No          | Integration routing rules                                             |

---

### Track Message

```json
{
  "type": "track",
  "messageId": "uuid",
  "userId": "user-123",
  "anonymousId": "anon-456",
  "event": "Order Completed",
  "timestamp": "2024-01-15T10:30:00Z",
  "properties": {
    "revenue": 99.99,
    "currency": "USD",
    "orderId": "order-789",
    "products": [
      { "id": "prod-1", "sku": "SKU-001", "name": "Widget", "price": 49.99 }
    ]
  },
  "context": { ... },
  "integrations": { "All": true, "Mixpanel": false }
}
```

**Required**: `event`, and either `userId` or `anonymousId`

---

### Identify Message

```json
{
  "type": "identify",
  "messageId": "uuid",
  "userId": "user-123",
  "anonymousId": "anon-456",
  "timestamp": "2024-01-15T10:30:00Z",
  "traits": {
    "email": "user@example.com",
    "firstName": "John",
    "lastName": "Doe",
    "phone": "+1234567890",
    "age": 30,
    "birthday": "1994-01-15T00:00:00Z",
    "createdAt": "2023-01-01T00:00:00Z",
    "avatar": "https://example.com/avatar.png"
  },
  "context": { ... },
  "integrations": { ... }
}
```

**Required**: Either `userId` or `anonymousId`

---

### Page Message

```json
{
  "type": "page",
  "messageId": "uuid",
  "userId": "user-123",
  "anonymousId": "anon-456",
  "name": "Home",
  "timestamp": "2024-01-15T10:30:00Z",
  "properties": {
    "url": "https://example.com/home",
    "path": "/home",
    "title": "Home Page",
    "referrer": "https://google.com"
  },
  "context": { ... },
  "integrations": { ... }
}
```

**Required**: Either `userId` or `anonymousId`

---

### Screen Message

```json
{
  "type": "screen",
  "messageId": "uuid",
  "userId": "user-123",
  "anonymousId": "anon-456",
  "name": "Dashboard",
  "timestamp": "2024-01-15T10:30:00Z",
  "properties": {
    "category": "Main"
  },
  "context": { ... },
  "integrations": { ... }
}
```

**Required**: Either `userId` or `anonymousId`

---

### Group Message

```json
{
  "type": "group",
  "messageId": "uuid",
  "userId": "user-123",
  "anonymousId": "anon-456",
  "groupId": "company-789",
  "timestamp": "2024-01-15T10:30:00Z",
  "traits": {
    "name": "Acme Corp",
    "website": "https://acme.com",
    "description": "Enterprise software company"
  },
  "context": { ... },
  "integrations": { ... }
}
```

**Required**: `groupId`, and either `userId` or `anonymousId`

---

### Alias Message

```json
{
  "type": "alias",
  "messageId": "uuid",
  "userId": "new-user-id",
  "previousId": "old-anonymous-id",
  "timestamp": "2024-01-15T10:30:00Z",
  "context": { ... },
  "integrations": { ... }
}
```

**Required**: `userId`, `previousId`

---

## Context Object Schema

The context object can appear at both batch and message level:

```json
{
  "app": {
    "name": "MyApp",
    "version": "2.0.0",
    "build": "123",
    "namespace": "com.example.myapp"
  },
  "campaign": {
    "name": "Winter Sale",
    "source": "google",
    "medium": "cpc",
    "term": "winter deals",
    "content": "ad-variation-1"
  },
  "device": {
    "id": "device-uuid",
    "manufacturer": "Apple",
    "model": "iPhone 15",
    "name": "John's iPhone",
    "type": "mobile",
    "version": "17.0",
    "advertisingId": "ad-id"
  },
  "library": {
    "name": "origamy-go",
    "version": "3.0.0"
  },
  "location": {
    "city": "San Francisco",
    "country": "USA",
    "region": "California",
    "latitude": 37.7749,
    "longitude": -122.4194,
    "speed": 0
  },
  "network": {
    "bluetooth": false,
    "cellular": true,
    "wifi": false,
    "carrier": "Verizon"
  },
  "os": {
    "name": "iOS",
    "version": "17.0"
  },
  "page": {
    "hash": "#section",
    "path": "/products",
    "referrer": "https://google.com",
    "search": "?q=search",
    "title": "Products",
    "url": "https://example.com/products"
  },
  "referrer": {
    "type": "organic",
    "name": "Google",
    "url": "https://google.com",
    "link": "https://google.com/search?q=..."
  },
  "screen": {
    "density": 2,
    "width": 1920,
    "height": 1080
  },
  "ip": "192.168.1.1",
  "direct": false,
  "locale": "en-US",
  "groupId": "company-123",
  "timezone": "America/Los_Angeles",
  "userAgent": "Mozilla/5.0 ...",
  "traits": { ... }
}
```

---

## Integrations Object Schema

Controls which downstream integrations receive the event:

```json
{
  "All": true,
  "Mixpanel": true,
  "Salesforce": false,
  "Intercom": {
    "customField": "value"
  }
}
```

- `true`: Enable integration
- `false`: Disable integration
- Object: Enable with custom options

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
{
  "success": true
}
```

### Error Response

```json
{
  "success": false,
  "error": "Error description"
}
```

---

## Limits

| Limit                  | Value                            |
| ---------------------- | -------------------------------- |
| Max batch size         | 500,000 bytes (500KB)            |
| Max message size       | 32,000 bytes (32KB)              |
| Max messages per batch | No explicit limit (size-bounded) |
| Request timeout        | 10 seconds (SDK default)         |

---

## Full Example Request

```http
POST /v1/batch HTTP/1.1
Host: api.origamy.example.com
Authorization: Basic d3JpdGVfa2V5Xzk4NzY1NDMyMTA6
Content-Type: application/json
Content-Length: 1234
User-Agent: origamy-go (version: 3.0.0)

{
  "messageId": "550e8400-e29b-41d4-a716-446655440000",
  "sentAt": "2024-01-15T10:30:00Z",
  "context": {
    "library": {
      "name": "origamy-go",
      "version": "3.0.0"
    }
  },
  "batch": [
    {
      "type": "identify",
      "messageId": "msg-001",
      "userId": "user-123",
      "timestamp": "2024-01-15T10:30:00Z",
      "traits": {
        "email": "user@example.com",
        "firstName": "John"
      }
    },
    {
      "type": "track",
      "messageId": "msg-002",
      "userId": "user-123",
      "event": "Product Viewed",
      "timestamp": "2024-01-15T10:30:01Z",
      "properties": {
        "productId": "prod-456",
        "price": 29.99
      }
    }
  ]
}
```

---

## Validation Rules Summary

| Message Type | Required Fields                                             |
| ------------ | ----------------------------------------------------------- |
| `track`      | `type`, `messageId`, `event`, (`userId` OR `anonymousId`)   |
| `identify`   | `type`, `messageId`, (`userId` OR `anonymousId`)            |
| `page`       | `type`, `messageId`, (`userId` OR `anonymousId`)            |
| `screen`     | `type`, `messageId`, (`userId` OR `anonymousId`)            |
| `group`      | `type`, `messageId`, `groupId`, (`userId` OR `anonymousId`) |
| `alias`      | `type`, `messageId`, `userId`, `previousId`                 |
