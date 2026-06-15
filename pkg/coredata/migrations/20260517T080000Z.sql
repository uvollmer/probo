-- Copyright (c) 2026 Probo Inc <hello@getprobo.com>.
--
-- Permission to use, copy, modify, and/or distribute this software for any
-- purpose with or without fee is hereby granted, provided that the above
-- copyright notice and this permission notice appear in all copies.
--
-- THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
-- REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
-- AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
-- INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
-- LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
-- OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
-- PERFORMANCE OF THIS SOFTWARE.

CREATE TYPE device_platform AS ENUM (
    'DARWIN',
    'LINUX',
    'FREEBSD',
    'WINDOWS'
);

CREATE TYPE device_posture_status AS ENUM (
    'PASS',
    'FAIL',
    'UNKNOWN',
    'NOT_APPLICABLE'
);

CREATE TYPE device_state AS ENUM (
    'PENDING',
    'ACTIVE',
    'REVOKED'
);

CREATE TABLE devices (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON UPDATE CASCADE ON DELETE CASCADE,
    state device_state NOT NULL DEFAULT 'PENDING',
    hardware_uuid TEXT,
    serial_number TEXT,
    hostname TEXT,
    platform device_platform,
    os_version TEXT,
    agent_version TEXT,
    api_key_hash BYTEA NOT NULL,
    owner_id TEXT,
    labels JSONB NOT NULL DEFAULT '{}'::jsonb,
    enrolled_at TIMESTAMP WITH TIME ZONE,
    last_seen_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    CONSTRAINT devices_active_fields_check CHECK (
        state != 'ACTIVE'
        OR (
            hardware_uuid IS NOT NULL
            AND hostname IS NOT NULL
            AND platform IS NOT NULL
            AND os_version IS NOT NULL
            AND agent_version IS NOT NULL
            AND enrolled_at IS NOT NULL
            AND last_seen_at IS NOT NULL
        )
    ),
    CONSTRAINT devices_revoked_at_check CHECK (
        (state = 'REVOKED' AND revoked_at IS NOT NULL)
        OR (state != 'REVOKED' AND revoked_at IS NULL)
    )
);

CREATE UNIQUE INDEX devices_org_hardware_uuid_idx
    ON devices (organization_id, hardware_uuid)
    WHERE hardware_uuid IS NOT NULL;

CREATE UNIQUE INDEX devices_api_key_hash_idx
    ON devices (api_key_hash);

CREATE INDEX devices_owner_idx
    ON devices (owner_id)
    WHERE owner_id IS NOT NULL;

CREATE TABLE device_postures (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON UPDATE CASCADE ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON UPDATE CASCADE ON DELETE CASCADE,
    check_key TEXT NOT NULL,
    status device_posture_status NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    observed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX device_postures_device_id_check_key_observed_at_idx
    ON device_postures (device_id, check_key, observed_at DESC);

CREATE INDEX device_postures_organization_id_idx
    ON device_postures (organization_id);
