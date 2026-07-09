// Copyright (c) 2025-2026 Probo Inc <hello@probo.com>.
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

package visitor

import (
	"context"
	"fmt"
	"time"

	"go.gearno.de/kit/pg"
	"go.probo.inc/probo/pkg/coredata"
	"go.probo.inc/probo/pkg/gid"
)

func (s *Service) GetFramework(
	ctx context.Context,
	scope coredata.Scoper,
	frameworkID gid.GID,
) (*coredata.Framework, error) {
	framework := &coredata.Framework{}

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			err := framework.LoadByID(ctx, conn, scope, frameworkID)
			if err != nil {
				return fmt.Errorf("cannot load framework: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return framework, nil
}

func (s *Service) GenerateFrameworkLightLogoURL(
	ctx context.Context,
	scope coredata.Scoper,
	frameworkID gid.GID,
	expiresIn time.Duration,
) (*string, error) {
	file := &coredata.File{}

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			framework := &coredata.Framework{}
			if err := framework.LoadByID(ctx, conn, scope, frameworkID); err != nil {
				return fmt.Errorf("cannot load framework: %w", err)
			}

			if framework.LightLogoFileID == nil {
				return nil
			}

			if err := file.LoadByID(ctx, conn, scope, *framework.LightLogoFileID); err != nil {
				return fmt.Errorf("cannot load file: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	if file.FileKey == "" {
		return nil, nil
	}

	presignedURL, err := s.fileManager.GeneratePresignedURL(ctx, file, expiresIn)
	if err != nil {
		return nil, fmt.Errorf("cannot generate file URL: %w", err)
	}

	return &presignedURL, nil
}

func (s *Service) GenerateFrameworkDarkLogoURL(
	ctx context.Context,
	scope coredata.Scoper,
	frameworkID gid.GID,
	expiresIn time.Duration,
) (*string, error) {
	file := &coredata.File{}

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			framework := &coredata.Framework{}
			if err := framework.LoadByID(ctx, conn, scope, frameworkID); err != nil {
				return fmt.Errorf("cannot load framework: %w", err)
			}

			if framework.DarkLogoFileID == nil {
				return nil
			}

			if err := file.LoadByID(ctx, conn, scope, *framework.DarkLogoFileID); err != nil {
				return fmt.Errorf("cannot load file: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	if file.FileKey == "" {
		return nil, nil
	}

	presignedURL, err := s.fileManager.GeneratePresignedURL(ctx, file, expiresIn)
	if err != nil {
		return nil, fmt.Errorf("cannot generate file URL: %w", err)
	}

	return &presignedURL, nil
}
