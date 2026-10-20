/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */
package health

// Exported aliases of internal helpers, used by the unit tests under test/unit-test/health.
var (
	SpyreHealthSocket  = spyreHealthSocket
	LoadTLSCredentials = loadTLSCredentials // pragma: allowlist secret
)

// SetHealths overwrites the client's cached health info map. For testing.
func (t *SpyreHealthClient) SetHealths(healthInfoMap map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if healthInfoMap != nil {
		t.healthInfoMap = healthInfoMap
	}
}
