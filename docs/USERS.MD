# CineVault User System

This document describes how CineVault manages users, authentication, households, parental controls, and library access.

---

## User Model

Each user is a row in the `users` table with the following fields:

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Primary key |
| `username` | string | Unique login name |
| `email` | string | Email address (unique) |
| `password_hash` | string | bcrypt hash (never exposed via API) |
| `pin_hash` | string (nullable) | bcrypt hash of optional PIN |
| `display_name` | string (nullable) | Friendly name shown in UI |
| `first_name` | string (nullable) | First name |
| `last_name` | string (nullable) | Last name |
| `role` | enum | `admin`, `user`, or `guest` |
| `is_active` | boolean | Whether the account can log in |
| `max_content_rating` | string (nullable) | Parental control cap (NULL = unrestricted) |
| `is_kids_profile` | boolean | Kids mode flag |
| `avatar_id` | string (nullable) | Pre-built avatar identifier |
| `parent_user_id` | UUID (nullable) | NULL = master user; set = sub-profile belonging to that master |
| `created_at` | timestamp | Account creation time |
| `updated_at` | timestamp | Last modification time |

Two computed fields are derived at query time and never stored:
- `has_pin` — true when `pin_hash` is non-empty
- `is_master` — true when `parent_user_id` is NULL

---

## Roles and Permissions

CineVault uses a numeric hierarchy for role checks:

| Role | Level | Description |
|---|---|---|
| `guest` | 1 | Lowest access level |
| `user` | 2 | Standard authenticated user |
| `admin` | 3 | Full system access |

An endpoint that requires `user` level is accessible to both `user` and `admin` roles. An endpoint that requires `admin` is restricted to admins only. The middleware enforces this on every authenticated request.

---

## Authentication

### JWT Tokens

All authenticated requests require a Bearer token in the `Authorization` header (or a `?token=` query parameter for streaming endpoints).

Token claims:
- `user_id` — UUID of the authenticated user
- `username` — username string
- `role` — the user's role
- `exp` — expiration timestamp
- `iat` — issued-at timestamp

Tokens are signed with HS256 using a configurable secret (`JWT_SECRET`). Expiration is configurable via `JWT_EXPIRES_IN` (e.g. `24h`).

### Password Authentication

Standard login sends `{username, password}` to `POST /api/v1/auth/login`. The server verifies the bcrypt hash and returns a JWT + user object.

### PIN Authentication (Fast Login)

Fast login is a streamlined flow for shared devices (TV, kiosk). When enabled via the `fast_login_enabled` setting:

1. The UI calls `GET /api/v1/auth/fast-login/users` to fetch the list of master users
2. The user taps their avatar and enters their PIN
3. The UI sends `{user_id, pin}` to `POST /api/v1/auth/fast-login`
4. The server verifies the bcrypt-hashed PIN and returns a JWT

PINs are numeric-only, with length controlled by the `fast_login_pin_length` setting (default 4).

### First-Run Setup

When no users exist in the database, the setup wizard is triggered:

1. `GET /api/v1/setup/check` returns `{setup_required: true}`
2. The UI shows a setup form
3. `POST /api/v1/setup` creates the first user as an admin
4. The response includes a JWT so the user is immediately logged in

### Two-Factor Authentication (TOTP)

CineVault supports time-based one-time password (TOTP) two-factor authentication compatible with any authenticator app (Google Authenticator, Authy, etc.).

**Setup flow:**

1. User calls `POST /api/v1/auth/2fa/setup` — server generates a TOTP secret and returns a provisioning URI (for QR code scanning)
2. User scans the QR code with their authenticator app
3. User calls `POST /api/v1/auth/2fa/verify` with a TOTP code from the app to confirm setup
4. 2FA is now enabled for the account

**Login flow with 2FA:**

1. User logs in with username/password as normal
2. If 2FA is enabled, the login response indicates a second factor is required
3. User calls `POST /api/v1/auth/2fa/validate` with the current TOTP code
4. On success, the full JWT is issued

**Management:**

- `GET /api/v1/auth/2fa/status` — check whether 2FA is enabled for the current user
- `DELETE /api/v1/auth/2fa` — disable 2FA (requires current TOTP code for confirmation)

### Password Reset

CineVault uses an admin-generated token system for password resets (no email required).

**Flow:**

1. Admin calls `POST /api/v1/auth/reset-token` with the target user's ID
2. Server generates a 32-byte hex token, stores a SHA-256 hash with a 24-hour expiry
3. Admin shares the token with the user out-of-band
4. User calls `POST /api/v1/auth/reset-password` with the token and their new password
5. Server validates the token, updates the password (bcrypt), marks the token as used, and invalidates all existing sessions for the user

Tokens are single-use and expire after 24 hours. The SHA-256 hash ensures that even if the database is compromised, tokens cannot be extracted.

### Session Management

Users can view and revoke their active sessions:

- `GET /api/v1/auth/sessions` — list all active sessions for the current user
- `DELETE /api/v1/auth/sessions/{id}` — revoke a specific session
- `POST /api/v1/auth/logout` — end the current session

---

## Household / Master-User System

CineVault uses a master/sub-profile architecture inspired by Netflix.

### Concepts

- **Master user** — a user with `parent_user_id = NULL`. Master users appear on the fast login screen and can log in independently.
- **Sub-profile** — a user with `parent_user_id` pointing to a master user. Sub-profiles cannot log in directly (they have an unusable password hash). They are only accessible after the master user has authenticated.
- A master user can have up to **5 sub-profiles**.
- All existing users are master users by default (backward-compatible).

### Authentication Flow

```
1. App loads → Fast Login screen shows master users only
2. Master enters PIN/password → JWT issued for master
3. "Who's Watching?" picker appears showing all household profiles
4. User taps a profile:
   - No PIN → immediate switch (new JWT issued)
   - Has PIN → inline PIN entry, then switch
5. App loads as the selected profile
```

If the master has no sub-profiles, the picker is skipped and the app loads directly.

### Profile Switching

Once inside the app, any household member can switch profiles via the avatar dropdown → "Switch Profile". This calls the same household endpoints without logging out.

The switch endpoint (`POST /api/v1/household/switch`) validates:
- Both profiles are in the same household
- The target profile is active
- The target's PIN (if set)

A new JWT is issued for the target profile.

### Sub-Profile Creation

Masters can create sub-profiles via "Manage Profiles" in the avatar dropdown. Each sub-profile gets:
- A system-generated username (`{master}_profile{N}`)
- A dummy email (`{master}_profile{N}@household.local`)
- An unusable password hash (prevents direct login)
- Optional PIN, avatar, kids mode, and content rating settings

### Deletion

Deleting a master user cascades to all their sub-profiles (enforced by `ON DELETE CASCADE` at the database level). Deleting a sub-profile only removes that profile.

---

## Parental Controls

Each user/profile has an optional `max_content_rating` field that caps the content they can see.

### Supported Ratings

| Rating | Level |
|---|---|
| G / TV-Y | 1 |
| PG / TV-Y7 / TV-G | 2 |
| PG-13 / TV-PG | 3 |
| R / TV-14 | 4 |
| NC-17 / TV-MA | 5 |

When `max_content_rating` is set, the recommendations endpoint filters media to only return items rated at or below that level. A NULL value means unrestricted.

### Kids Mode

Setting `is_kids_profile = true` enables a simplified UI with a "Kids Mode Active" banner. If kids mode is turned on and no content rating is set, the system automatically defaults to `PG`.

---

## Avatars

Users can pick from a set of pre-built avatars identified by string IDs (e.g. `av-lion`, `av-panda`). Each avatar has an emoji and gradient background rendered in the UI. If no avatar is selected, a color-coded initial circle is generated from the user's name.

Avatars appear on:
- The fast login screen
- The "Who's Watching?" picker
- The profile switch overlay
- The top-bar avatar button
- The Manage Profiles list

---

## Library Access

Libraries have three access levels:

| Level | Behavior |
|---|---|
| `everyone` | All authenticated users can access |
| `select_users` | Only users with explicit permission |
| `admin_only` | Only admin users |

Permissions are stored in the `library_permissions` table, linking a `library_id` to a `user_id`. Admins always see all libraries regardless of access level.

The access check runs on every library-related request (listing libraries, browsing media, streaming, recommendations).

Sub-profiles are checked using their own user ID — they do not automatically inherit their master's library permissions. A master user must grant library access to each sub-profile separately via the admin settings.

---

## Streaming Limits

Admins can set per-user streaming limits to control resource usage:

| Field | Description |
|---|---|
| `max_streams` | Maximum concurrent streams allowed for the user |
| `max_bitrate` | Maximum bitrate cap (bits per second) |
| `remote_quality_cap` | Quality cap for remote (non-local) streaming |

Streaming limits are managed via admin-only endpoints and enforced by the stream handlers.

---

## Profile Statistics

Users can view their profile statistics:

- `GET /api/v1/profile/stats` — returns aggregate stats: total items watched, total watch time, favorite genres, most-watched actors/directors
- `GET /api/v1/profile/wrapped/{year}` — returns a "Wrapped" summary for a given year (similar to Spotify Wrapped): top genres, top titles, total hours, viewing patterns

---

## API Endpoints

### Public (No Auth)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/setup/check` | Check if first-run setup is needed |
| POST | `/api/v1/setup` | Create initial admin user |
| POST | `/api/v1/auth/register` | Register new user |
| POST | `/api/v1/auth/login` | Standard login |
| GET | `/api/v1/auth/fast-login/settings` | Fast login configuration |
| GET | `/api/v1/auth/fast-login/users` | Master users for fast login |
| POST | `/api/v1/auth/fast-login` | PIN-based login |
| POST | `/api/v1/auth/reset-password` | Reset password with admin-generated token |

### Authenticated (User)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/profile` | Get own profile |
| PUT | `/api/v1/profile` | Update own profile (name, email, password) |
| PUT | `/api/v1/auth/pin` | Set own PIN |
| GET | `/api/v1/profile/settings` | Get parental controls, kids mode, avatar |
| PUT | `/api/v1/profile/settings` | Update parental controls, kids mode, avatar |
| GET | `/api/v1/profile/stats` | Profile viewing statistics |
| GET | `/api/v1/profile/wrapped/{year}` | Yearly wrapped summary |
| POST | `/api/v1/auth/logout` | End current session |
| GET | `/api/v1/auth/sessions` | List active sessions |
| DELETE | `/api/v1/auth/sessions/{id}` | Revoke a session |
| POST | `/api/v1/auth/2fa/setup` | Generate TOTP secret for 2FA setup |
| POST | `/api/v1/auth/2fa/verify` | Confirm 2FA setup with TOTP code |
| POST | `/api/v1/auth/2fa/validate` | Validate TOTP code during login |
| GET | `/api/v1/auth/2fa/status` | Check 2FA enabled status |
| DELETE | `/api/v1/auth/2fa` | Disable 2FA |
| GET | `/api/v1/household/profiles` | List household profiles |
| POST | `/api/v1/household/switch` | Switch to another household profile |
| POST | `/api/v1/household/profiles` | Create sub-profile (master only) |
| PUT | `/api/v1/household/profiles/{id}` | Update sub-profile (master only) |
| DELETE | `/api/v1/household/profiles/{id}` | Delete sub-profile (master only) |

### Admin Only

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/users` | List all users |
| PUT | `/api/v1/users/{id}/pin` | Set PIN for any user |
| PUT | `/api/v1/users/{id}/settings` | Update any user's profile settings |
| POST | `/api/v1/auth/reset-token` | Generate password reset token for a user |
| GET | `/api/v1/admin/users/{id}/stream-limits` | Get user streaming limits |
| PUT | `/api/v1/admin/users/{id}/stream-limits` | Update user streaming limits |

---

## Settings

| Key | Default | Description |
|---|---|---|
| `fast_login_enabled` | `false` | Enable the fast login / PIN screen |
| `fast_login_pin_length` | `4` | Required PIN digit count |
| `login_intro_enabled` | `true` | Play intro animation on login |
| `login_intro_muted` | `false` | Mute intro video audio |

---

## Database Migrations

| Migration | Description |
|---|---|
| 004 | Adds `library_access` enum, `access_level` column on libraries, `library_permissions` table |
| 012 | Adds `pin_hash` and `display_name` columns for fast login support |
| 013 | Adds `login_intro_enabled` and `login_intro_muted` settings |
| 014 | Adds `first_name` and `last_name` columns to users |
| 019 | Adds `max_content_rating`, `is_kids_profile`, `avatar_id` columns to users |
| 020 | Adds `parent_user_id` column with FK + index for household profiles |
| 021 | Adds `mood` to tag category enum, `rules` JSONB on collections, `keywords` on media items, recommendation indexes |
