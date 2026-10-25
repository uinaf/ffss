Incident write-up and the fixed code are below. Unit tests were green the whole
time and still are. I don't want another test that happens to hit the right
timing; I want this class of bug proven in this repo, in a way the next agent
who touches auth will keep up. Set it up. If you can't run a tool here, say so
rather than guessing its output.

======= FILE: docs/incidents/2026-09-session-logouts.md =======
# Random logouts after app resume (September 2026)

Users were signed out a few seconds after resuming the app.

Cause: on resume, the files list and the transfers poller both got 401 with
the expired access token and both called `TokenRefresher.refresh()`. The API
rotates refresh tokens: each refresh returns a new refresh token and revokes
the old one. The second refresh sent the already-revoked token, the API
answered `invalid_grant`, and the app treated that as a sign-out.

Fix (`a41c9e2`): refreshes are serialized behind a mutex, and a caller whose
failed request used an access token that has since been replaced reuses the
new token instead of refreshing again.
======= END FILE =======

======= FILE: app/src/main/kotlin/io/tidal/auth/TokenRefresher.kt =======
package io.tidal.auth

import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

data class Tokens(val access: String, val refresh: String)

sealed interface RefreshResult {
    data class Refreshed(val tokens: Tokens) : RefreshResult
    data object SignedOut : RefreshResult
}

interface AuthApi {
    /** Returns new tokens, or null on invalid_grant. Rotates the refresh token. */
    suspend fun refresh(refreshToken: String): Tokens?
}

class TokenRefresher(
    private val api: AuthApi,
    private val store: TokenStore,
) {
    private val mutex = Mutex()

    /** Called by any request that got 401 while sending [failedAccess]. */
    suspend fun refresh(failedAccess: String): RefreshResult = mutex.withLock {
        val current = store.read() ?: return RefreshResult.SignedOut
        if (current.access != failedAccess) {
            return RefreshResult.Refreshed(current)
        }
        val next = api.refresh(current.refresh)
        if (next == null) {
            store.clear()
            return RefreshResult.SignedOut
        }
        store.write(next)
        RefreshResult.Refreshed(next)
    }
}

interface TokenStore {
    fun read(): Tokens?
    fun write(tokens: Tokens)
    fun clear()
}
======= END FILE =======

======= FILE: settings.gradle.kts =======
rootProject.name = "tidal"
include(":app")
======= END FILE =======

======= FILE: .github/workflows/ci.yml =======
name: CI
on: [push, pull_request]
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-java@v5
        with: { distribution: temurin, java-version: "21" }
      - run: ./gradlew check
======= END FILE =======
