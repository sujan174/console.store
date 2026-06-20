# Data model

Postgres (prod) / SQLite (dev). Tokens encrypted at rest. Curation is editorial content owned by console.store.

## Entities

### account
| col | type | notes |
|-----|------|-------|
| phone | text PK | primary key (from Swiggy JWT `phone` claim) |
| swiggy_sub | text | JWT `sub`, fallback identity |
| created_at | timestamptz | |
| last_seen_at | timestamptz | |

### device
| col | type | notes |
|-----|------|-------|
| id | uuid PK | |
| phone | text FK → account | |
| ssh_pubkey | text unique | bound after phone verification |
| label | text | e.g. "macbook", "work" |
| bound_at | timestamptz | |

### swiggy_token
| col | type | notes |
|-----|------|-------|
| phone | text FK → account | one active token per user |
| ciphertext | bytea | encrypted JWT (`store/crypto.go`) |
| expires_at | timestamptz | JWT exp (≈ now + 5d) |
| session_expires_at | timestamptz | 30-day idle window |
| updated_at | timestamptz | |

### address (cache of Swiggy addresses)
| col | type | notes |
|-----|------|-------|
| id | text PK | Swiggy address id |
| phone | text FK | |
| label | text | home / work / … |
| city | text | drives curation whitelist |
| raw | jsonb | full Swiggy address |

### order_history
| col | type | notes |
|-----|------|-------|
| id | uuid PK | |
| phone | text FK | |
| swiggy_order_id | text | |
| server | text | `food` \| `instamart` |
| items | jsonb | snapshot for "the usual" |
| total | int | paise |
| placed_at | timestamptz | |

## Curation content (operator-owned, versioned)

### curated_restaurant
| col | type | notes |
|-----|------|-------|
| id | text PK | console.store id |
| city | text | |
| swiggy_name | text | match key against live `search_restaurants` |
| category | text | coffee / food / snacks |
| editorial_note | text | optional blurb |
| active | bool | |

### curated_item
| col | type | notes |
|-----|------|-------|
| id | text PK | |
| curated_restaurant_id | text FK | |
| swiggy_item_name | text | match key in `get_restaurant_menu` |
| tags | text[] | `fav`, `new` |
| sort | int | editorial ordering |

### curated_instamart_sku
| col | type | notes |
|-----|------|-------|
| spin_id | text PK | Swiggy Instamart `spinId` |
| city | text | |
| name | text | |
| tags | text[] | |

## Matching strategy (curation ∩ live)

Live Swiggy results are matched to curation rows by **name + city** (normalized). A curated entry only renders if Swiggy reports it serviceable + `OPEN` at the user's address. Unmatched curated rows are silently hidden (logged for ops to fix coverage).

## Encryption

- `swiggy_token.ciphertext` = AES-GCM with a data key from KMS (envelope encryption); nonce stored alongside.
- Decryption happens only in-memory inside `internal/swiggy` at call time.
- Rotation: re-encrypt on data-key rotation; tokens are short-lived (5d) so rotation impact is small.

## "The usual"

`curation.Usual(history)` = most-frequent / most-recent item across `order_history` for that phone, resolved to a `curated_item` if still curated + serviceable. Rendered as the pinned top row.
```
