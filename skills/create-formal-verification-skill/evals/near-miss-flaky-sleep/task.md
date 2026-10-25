This test fails about 1 run in 20 on CI and never locally. Fix it.

======= FILE: app/src/test/kotlin/io/tidal/sync/SyncQueueTest.kt =======
package io.tidal.sync

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlin.test.Test
import kotlin.test.assertEquals

class SyncQueueTest {
    @Test
    fun drainsAllQueuedItems() {
        val sent = mutableListOf<String>()
        val queue = SyncQueue(CoroutineScope(Dispatchers.Default)) { sent += it }
        queue.enqueue("a")
        queue.enqueue("b")
        Thread.sleep(100)
        assertEquals(listOf("a", "b"), sent)
    }
}
======= END FILE =======

======= FILE: app/src/main/kotlin/io/tidal/sync/SyncQueue.kt =======
package io.tidal.sync

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch

class SyncQueue(scope: CoroutineScope, private val send: suspend (String) -> Unit) {
    private val items = Channel<String>(Channel.UNLIMITED)

    init {
        scope.launch { for (item in items) send(item) }
    }

    fun enqueue(item: String) {
        items.trySend(item)
    }
}
======= END FILE =======
