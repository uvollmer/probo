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
	"net/url"
	"path/filepath"
	"time"

	"go.gearno.de/kit/pg"
	"go.probo.inc/probo/packages/emails"
	"go.probo.inc/probo/pkg/complianceportal"
	"go.probo.inc/probo/pkg/coredata"
	"go.probo.inc/probo/pkg/gid"
)

func (s *Service) GetPortal(
	ctx context.Context,
	scope coredata.Scoper,
	trustCenterID gid.GID,
) (*coredata.TrustCenter, error) {
	var trustCenter *coredata.TrustCenter

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			trustCenter = &coredata.TrustCenter{}
			if err := trustCenter.LoadByID(ctx, conn, scope, trustCenterID); err != nil {
				return fmt.Errorf("cannot load trust center: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot load trust center: %w", err)
	}

	return trustCenter, nil
}

func (s *Service) GetPortalByOrganizationID(
	ctx context.Context,
	scope coredata.Scoper,
	organizationID gid.GID,
) (*coredata.TrustCenter, error) {
	trustCenter := &coredata.TrustCenter{}

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			err := trustCenter.LoadByOrganizationID(ctx, conn, scope, organizationID)
			if err != nil {
				return fmt.Errorf("cannot load trust center: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return trustCenter, nil
}

func (s *Service) GetPortalNDAFile(
	ctx context.Context,
	scope coredata.Scoper,
	trustCenterID gid.GID,
) (*coredata.File, error) {
	var file *coredata.File

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			trustCenter := &coredata.TrustCenter{}
			if err := trustCenter.LoadByID(ctx, conn, scope, trustCenterID); err != nil {
				return fmt.Errorf("cannot load trust center: %w", err)
			}

			if trustCenter.NonDisclosureAgreementFileID == nil {
				return nil
			}

			file = &coredata.File{}
			if err := file.LoadByID(ctx, conn, scope, *trustCenter.NonDisclosureAgreementFileID); err != nil {
				return fmt.Errorf("cannot load file: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (s *Service) GeneratePortalNDAFileURL(
	ctx context.Context,
	scope coredata.Scoper,
	trustCenterID gid.GID,
	expiresIn time.Duration,
) (string, error) {
	var file *coredata.File

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			trustCenter := &coredata.TrustCenter{}
			if err := trustCenter.LoadByID(ctx, conn, scope, trustCenterID); err != nil {
				return fmt.Errorf("cannot load trust center: %w", err)
			}

			if trustCenter.NonDisclosureAgreementFileID == nil {
				return fmt.Errorf("no NDA file found")
			}

			file = &coredata.File{}
			if err := file.LoadByID(ctx, conn, scope, *trustCenter.NonDisclosureAgreementFileID); err != nil {
				return fmt.Errorf("cannot load file: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return "", err
	}

	presignedURL, err := s.fileManager.GeneratePresignedURL(ctx, file, expiresIn)
	if err != nil {
		return "", fmt.Errorf("cannot generate file URL: %w", err)
	}

	return presignedURL, nil
}

func (s *Service) GeneratePortalLogoURL(
	ctx context.Context,
	scope coredata.Scoper,
	compliancePageID gid.GID,
	expiresIn time.Duration,
) (*string, error) {
	file := &coredata.File{}
	compliancePage := &coredata.TrustCenter{}

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := compliancePage.LoadByID(ctx, conn, scope, compliancePageID); err != nil {
				return fmt.Errorf("cannot load compliance page: %w", err)
			}

			if compliancePage.LogoFileID == nil {
				return nil
			}

			if err := file.LoadByID(ctx, conn, scope, *compliancePage.LogoFileID); err != nil {
				return fmt.Errorf("cannot load file: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	if compliancePage.LogoFileID == nil {
		return nil, nil
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

func (s *Service) GeneratePortalDarkLogoURL(
	ctx context.Context,
	scope coredata.Scoper,
	compliancePageID gid.GID,
	expiresIn time.Duration,
) (*string, error) {
	file := &coredata.File{}
	compliancePage := &coredata.TrustCenter{}

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := compliancePage.LoadByID(ctx, conn, scope, compliancePageID); err != nil {
				return fmt.Errorf("cannot load compliance page: %w", err)
			}

			if compliancePage.DarkLogoFileID == nil {
				return nil
			}

			if err := file.LoadByID(ctx, conn, scope, *compliancePage.DarkLogoFileID); err != nil {
				return fmt.Errorf("cannot load file: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	if compliancePage.DarkLogoFileID == nil {
		return nil, nil
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

func (s *Service) GetPortalEmailPresenterConfig(
	ctx context.Context,
	scope coredata.Scoper,
	compliancePageID gid.GID,
) (emails.PresenterConfig, error) {
	var (
		compliancePage    = &coredata.TrustCenter{}
		organization      = &coredata.Organization{}
		customDomain      *coredata.CustomDomain
		logoFile          = &coredata.File{}
		emailPresenterCfg = emails.DefaultPresenterConfig(s.baseURL)
	)

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := compliancePage.LoadByID(ctx, conn, scope, compliancePageID); err != nil {
				return fmt.Errorf("cannot load compliance page: %w", err)
			}

			if compliancePage.LogoFileID != nil {
				if err := logoFile.LoadByID(ctx, conn, scope, *compliancePage.LogoFileID); err != nil {
					return fmt.Errorf("cannot load logoFile: %w", err)
				}
			}

			if err := organization.LoadByID(ctx, conn, scope, compliancePage.OrganizationID); err != nil {
				return fmt.Errorf("cannot load organization: %w", err)
			}

			effective, err := complianceportal.EffectiveDomainForTrustCenter(ctx, conn, scope, compliancePage)
			if err != nil {
				return fmt.Errorf("cannot resolve effective custom domain: %w", err)
			}

			customDomain = effective

			return nil
		},
	)
	if err != nil {
		return emailPresenterCfg, err
	}

	parsedBaseURL, err := url.Parse(s.baseURL)
	if err != nil {
		return emailPresenterCfg, fmt.Errorf("cannot parse base URL: %w", err)
	}

	baseURL := url.URL{
		Scheme: parsedBaseURL.Scheme,
		Host:   parsedBaseURL.Host,
		Path:   "/trust/" + compliancePage.Slug,
	}

	if customDomain != nil {
		baseURL.Host = customDomain.Domain
		baseURL.Scheme = "https"
		baseURL.Path = ""
	}

	emailPresenterCfg.BaseURL = baseURL.String()

	if compliancePage.LogoFileID != nil {
		if logoFile.FileKey == "" {
			return emailPresenterCfg, nil
		}

		// If logo exists, then we will brand the emails with the org as a sender
		emailPresenterCfg.SenderCompanyLogoPath = filepath.Join("/api/files/v1/public/", logoFile.ID.String())
		emailPresenterCfg.SenderCompanyName = organization.Name

		if organization.WebsiteURL != nil {
			emailPresenterCfg.SenderCompanyWebsiteURL = *organization.WebsiteURL
		}

		if organization.HeadquarterAddress != nil {
			emailPresenterCfg.SenderCompanyHeadquarterAddress = *organization.HeadquarterAddress
		}
	}

	return emailPresenterCfg, nil
}

func (s *Service) GetPortalMailingList(
	ctx context.Context,
	scope coredata.Scoper,
	trustCenterID gid.GID,
) (*coredata.MailingList, error) {
	var mailingList *coredata.MailingList

	err := s.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			trustCenter := &coredata.TrustCenter{}
			if err := trustCenter.LoadByID(ctx, conn, scope, trustCenterID); err != nil {
				return fmt.Errorf("cannot load trust center: %w", err)
			}

			if trustCenter.MailingListID == nil {
				return nil
			}

			mailingList = &coredata.MailingList{}
			if err := mailingList.LoadByID(ctx, conn, scope, *trustCenter.MailingListID); err != nil {
				return fmt.Errorf("cannot load mailing list: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return mailingList, nil
}
