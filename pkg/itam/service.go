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

package itam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.gearno.de/kit/log"
	"go.gearno.de/kit/pg"
	"go.probo.inc/probo/pkg/coredata"
	"go.probo.inc/probo/pkg/gid"
	"go.probo.inc/probo/pkg/iam"
	"go.probo.inc/probo/pkg/page"
)

var (
	// ErrDeviceRevoked is returned when the authenticated device has
	// been revoked.
	ErrDeviceRevoked = errors.New("device is revoked")

	// ErrDeviceHardwareConflict is returned when activation would
	// duplicate an existing (organization_id, hardware_uuid) pair.
	ErrDeviceHardwareConflict = errors.New("device hardware uuid already enrolled")
)

const (
	// APIKeyRawLength is the random byte length of a device API key
	// secret (64 chars once base64url-encoded).
	APIKeyRawLength = 48
)

type (
	// Service is the IT Asset Management service. Admin operations are
	// tenant-scoped via a caller-supplied scope; agent-facing operations
	// (authenticate, heartbeat, postures, unenroll) resolve their own
	// scope, since the agent does not know its tenant until activation.
	Service struct {
		pg     *pg.Client
		logger *log.Logger
	}

	CreateDeviceRequest struct {
		OrganizationID gid.GID
		OwnerID        *gid.GID
	}

	// CreateDeviceResult carries the device row and the plaintext API
	// key the agent must persist. Only the hash is stored, so APIKey is
	// available only at this point.
	CreateDeviceResult struct {
		Device *coredata.Device
		APIKey string
	}

	RecordHeartbeatRequest struct {
		HardwareUUID string
		SerialNumber *string
		Hostname     string
		Platform     coredata.DevicePlatform
		OSVersion    string
		AgentVersion string
	}

	RecordPostureResult struct {
		CheckKey   string
		Status     coredata.DevicePostureStatus
		Evidence   json.RawMessage
		ObservedAt time.Time
	}
)

func NewService(pgClient *pg.Client, iamSvc *iam.Service, logger *log.Logger) *Service {
	iamSvc.Authorizer.RegisterPolicySet(ITAMPolicySet())

	return &Service{
		pg:     pgClient,
		logger: logger,
	}
}

// hashSecret hashes a device API key for storage and lookup. Unsalted
// SHA-256 is sufficient: the input is a random secret with at least 256
// bits of entropy.
func hashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))

	return sum[:]
}

// generateSecret returns a base64url-encoded random secret of rawLen
// bytes.
func generateSecret(rawLen int) (string, error) {
	buf := make([]byte, rawLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("cannot generate random secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Service) CreateDevice(
	ctx context.Context,
	scope coredata.Scoper,
	req CreateDeviceRequest,
) (*CreateDeviceResult, error) {
	if req.OrganizationID == gid.Nil {
		return nil, fmt.Errorf("organization_id is required")
	}

	apiKey, err := generateSecret(APIKeyRawLength)
	if err != nil {
		return nil, err
	}

	apiKeyHash := hashSecret(apiKey)
	now := time.Now()

	device := &coredata.Device{
		ID:             gid.New(req.OrganizationID.TenantID(), coredata.DeviceEntityType),
		OrganizationID: req.OrganizationID,
		State:          coredata.DeviceStatePending,
		APIKeyHash:     apiKeyHash,
		OwnerID:        req.OwnerID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err = s.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			organization := &coredata.Organization{}
			if err := organization.LoadByID(ctx, conn, scope, req.OrganizationID); err != nil {
				return fmt.Errorf("cannot load organization: %w", err)
			}

			if err := device.Insert(ctx, conn, scope); err != nil {
				return fmt.Errorf("cannot insert device: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return &CreateDeviceResult{Device: device, APIKey: apiKey}, nil
}

func (s *Service) GetDevice(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
) (*coredata.Device, error) {
	device := &coredata.Device{}

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := device.LoadByID(ctx, conn, scope, deviceID); err != nil {
				return fmt.Errorf("cannot load device: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (s *Service) ListForOrganizationID(
	ctx context.Context,
	scope coredata.Scoper,
	organizationID gid.GID,
	cursor *page.Cursor[coredata.DeviceOrderField],
) (*page.Page[*coredata.Device, coredata.DeviceOrderField], error) {
	var devices coredata.Devices
	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := devices.LoadByOrganizationID(ctx, conn, scope, organizationID, cursor); err != nil {
				return fmt.Errorf("cannot load devices: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return page.NewPage(devices, cursor), nil
}

func (s *Service) CountForOrganizationID(
	ctx context.Context,
	scope coredata.Scoper,
	organizationID gid.GID,
) (int, error) {
	var count int

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			var ds coredata.Devices
			c, err := ds.CountByOrganizationID(ctx, conn, scope, organizationID)
			if err != nil {
				return fmt.Errorf("cannot count devices: %w", err)
			}

			count = c

			return nil
		},
	)

	return count, err
}

func (s *Service) RevokeDevice(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
) (*coredata.Device, error) {
	device := &coredata.Device{}

	err := s.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := device.LoadByID(ctx, conn, scope, deviceID); err != nil {
				return fmt.Errorf("cannot load device: %w", err)
			}

			if err := device.Revoke(ctx, conn, scope); err != nil {
				return fmt.Errorf("cannot revoke device: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (s *Service) AssignDeviceToUser(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
	identityID *gid.GID,
) (*coredata.Device, error) {
	device := &coredata.Device{}

	err := s.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := device.LoadByID(ctx, conn, scope, deviceID); err != nil {
				return fmt.Errorf("cannot load device: %w", err)
			}

			if err := device.AssignUser(ctx, conn, scope, identityID); err != nil {
				return fmt.Errorf("cannot assign device user: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (s *Service) GetLatestPostures(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
) (coredata.DevicePostures, error) {
	var postures coredata.DevicePostures

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := postures.LoadLatestByDeviceID(ctx, conn, scope, deviceID); err != nil {
				return fmt.Errorf("cannot load latest device postures: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return postures, nil
}

func (s *Service) GetPostureHistory(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
	checkKey string,
	limit int,
) (coredata.DevicePostures, error) {
	var postures coredata.DevicePostures

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := postures.LoadHistoryByDeviceIDAndCheckKey(ctx, conn, scope, deviceID, checkKey, limit); err != nil {
				return fmt.Errorf("cannot load device posture history: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return postures, nil
}

// AuthenticateDevice resolves a device API key to its device row.
// Returns coredata.ErrResourceNotFound when no device matches the key
// and ErrDeviceRevoked when the matching device has been revoked.
func (s *Service) AuthenticateDevice(
	ctx context.Context,
	apiKey string,
) (*coredata.Device, error) {
	if apiKey == "" {
		return nil, coredata.ErrResourceNotFound
	}

	hash := hashSecret(apiKey)

	device := &coredata.Device{}
	err := s.pg.WithConn(ctx, func(ctx context.Context, conn pg.Querier) error {
		return device.LoadByAPIKeyHash(ctx, conn, hash)
	})
	if err != nil {
		return nil, err
	}

	if device.State == coredata.DeviceStateRevoked {
		return nil, ErrDeviceRevoked
	}

	return device, nil
}

// RecordHeartbeat refreshes the device's last-seen timestamp and any
// version fields the agent sends. On the first heartbeat for a PENDING
// device, hardware metadata is recorded and the device is activated.
func (s *Service) RecordHeartbeat(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
	req RecordHeartbeatRequest,
) (*coredata.Device, error) {
	if req.HardwareUUID == "" {
		return nil, fmt.Errorf("hardware_uuid is required")
	}

	if req.Hostname == "" {
		return nil, fmt.Errorf("hostname is required")
	}

	if !req.Platform.IsValid() {
		return nil, fmt.Errorf("invalid platform: %q", req.Platform)
	}

	if req.OSVersion == "" {
		return nil, fmt.Errorf("os_version is required")
	}

	if req.AgentVersion == "" {
		return nil, fmt.Errorf("agent_version is required")
	}

	device := &coredata.Device{}

	err := s.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := device.LoadByID(ctx, conn, scope, deviceID); err != nil {
				return fmt.Errorf("cannot load device: %w", err)
			}

			if device.State == coredata.DeviceStateRevoked {
				return ErrDeviceRevoked
			}

			hardwareUUID := req.HardwareUUID
			hostname := req.Hostname
			platform := req.Platform
			osVersion := req.OSVersion
			agentVersion := req.AgentVersion

			device.HardwareUUID = &hardwareUUID
			device.SerialNumber = req.SerialNumber
			device.Hostname = &hostname
			device.Platform = &platform
			device.OSVersion = &osVersion
			device.AgentVersion = &agentVersion

			switch device.State {
			case coredata.DeviceStatePending:
				if err := device.Activate(ctx, conn, scope); err != nil {
					if errors.Is(err, coredata.ErrResourceAlreadyExists) {
						return ErrDeviceHardwareConflict
					}

					return fmt.Errorf("cannot activate device: %w", err)
				}
			case coredata.DeviceStateActive:
				if err := device.UpdateHeartbeat(ctx, conn, scope); err != nil {
					return fmt.Errorf("cannot update device heartbeat: %w", err)
				}
			default:
				return ErrDeviceRevoked
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return device, nil
}

// RecordPostures appends posture results for a device.
func (s *Service) RecordPostures(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
	results []RecordPostureResult,
) error {
	if len(results) == 0 {
		return nil
	}

	now := time.Now()

	return s.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			device := &coredata.Device{}
			if err := device.LoadByID(ctx, conn, scope, deviceID); err != nil {
				return fmt.Errorf("cannot load device: %w", err)
			}

			if device.State != coredata.DeviceStateActive {
				return ErrDeviceRevoked
			}

			for _, r := range results {
				observed := r.ObservedAt
				if observed.IsZero() {
					observed = now
				}

				posture := coredata.DevicePosture{
					ID:             gid.New(device.OrganizationID.TenantID(), coredata.DevicePostureEntityType),
					OrganizationID: device.OrganizationID,
					DeviceID:       device.ID,
					CheckKey:       r.CheckKey,
					Status:         r.Status,
					Evidence:       r.Evidence,
					ObservedAt:     observed,
					CreatedAt:      now,
				}
				if err := posture.Insert(ctx, conn, scope); err != nil {
					return fmt.Errorf("cannot insert device posture: %w", err)
				}
			}

			return nil
		},
	)
}

// UnenrollDevice revokes the device. The agent invokes this from its
// uninstaller before wiping its local API key.
func (s *Service) UnenrollDevice(
	ctx context.Context,
	scope coredata.Scoper,
	deviceID gid.GID,
) error {
	return s.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			device := &coredata.Device{}
			if err := device.LoadByID(ctx, conn, scope, deviceID); err != nil {
				return fmt.Errorf("cannot load device: %w", err)
			}

			if err := device.Revoke(ctx, conn, scope); err != nil {
				return fmt.Errorf("cannot revoke device: %w", err)
			}

			return nil
		},
	)
}
