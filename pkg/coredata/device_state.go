// Copyright (c) 2025-2026 Probo Inc <hello@getprobo.com>.
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
// REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
// AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
// INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
// LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
// OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
// PERFORMANCE OF THIS SOFTWARE.

package coredata

import "fmt"

type DeviceState string

const (
	DeviceStatePending DeviceState = "PENDING"
	DeviceStateActive  DeviceState = "ACTIVE"
	DeviceStateRevoked DeviceState = "REVOKED"
)

func (s DeviceState) String() string {
	return string(s)
}

func (s DeviceState) IsValid() bool {
	switch s {
	case DeviceStatePending, DeviceStateActive, DeviceStateRevoked:
		return true
	}

	return false
}

func (s DeviceState) MarshalText() ([]byte, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("invalid device state: %q", string(s))
	}

	return []byte(s), nil
}

func (s *DeviceState) UnmarshalText(text []byte) error {
	v := DeviceState(text)
	if !v.IsValid() {
		return fmt.Errorf("invalid device state: %q", string(text))
	}

	*s = v

	return nil
}
