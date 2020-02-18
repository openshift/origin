/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package gocertifi

import "testing"

func TestGetCerts(t *testing.T) {
	certPool, err := CACerts()
	if certPool == nil || err != nil || len(certPool.Subjects()) == 0 {
		t.Errorf("Failed to return the certificates.")
	}
}
