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

package certmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.gearno.de/kit/log"
	"go.gearno.de/kit/pg"
	"go.probo.inc/probo/pkg/coredata"
	"go.probo.inc/probo/pkg/crypto/cipher"
	"go.probo.inc/probo/pkg/gid"
)

type (
	Renewer struct {
		pg            *pg.Client
		acmeService   *ACMEService
		encryptionKey cipher.EncryptionKey
		interval      time.Duration
		logger        *log.Logger
	}
)

func NewRenewer(
	pg *pg.Client,
	acmeService *ACMEService,
	encryptionKey cipher.EncryptionKey,
	interval time.Duration,
	logger *log.Logger,
) *Renewer {
	return &Renewer{
		pg:            pg,
		acmeService:   acmeService,
		encryptionKey: encryptionKey,
		interval:      interval,
		logger:        logger.Named("certmanager.renewer"),
	}
}

func (r *Renewer) Run(ctx context.Context) error {
	r.logger.InfoCtx(ctx, "certificate renewer starting")

	if err := r.checkAndRenew(ctx); err != nil {
		r.logger.ErrorCtx(ctx, "cannot perform initial renewal check", log.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.InfoCtx(ctx, "certificate renewer shutting down")
			return ctx.Err()
		case <-time.After(r.interval):
			if err := r.checkAndRenew(ctx); err != nil {
				r.logger.ErrorCtx(ctx, "cannot perform renewal check", log.Error(err))
			}
		}
	}
}

func (r *Renewer) checkAndRenew(ctx context.Context) error {
	return r.pg.WithTx(
		ctx,
		func(ctx context.Context, tx pg.Tx) error {
			var caches coredata.CachedCertificates

			cacheCount, err := caches.CountAll(ctx, tx)
			if err != nil {
				r.logger.ErrorCtx(ctx, "cannot count certificate cache", log.Error(err))
			} else if cacheCount == 0 {
				r.logger.InfoCtx(ctx, "certificate cache is empty, rebuilding from certificates")

				warmer := NewCacheStore(r.pg, r.encryptionKey, r.logger)
				if err := warmer.WarmCache(ctx); err != nil {
					r.logger.ErrorCtx(ctx, "cannot rebuild certificate cache", log.Error(err))
				} else {
					r.logger.InfoCtx(ctx, "certificate cache rebuilt successfully")
				}
			}

			if err := caches.CleanExpired(ctx, tx); err != nil {
				r.logger.ErrorCtx(ctx, "cannot clean certificate cache", log.Error(err))
			}

			certificates := coredata.Certificates{}

			scope := coredata.NewNoScope()
			if err := certificates.ListForRenewal(ctx, tx, scope); err != nil {
				return fmt.Errorf("cannot list certificates for renewal: %w", err)
			}

			if len(certificates) == 0 {
				return nil
			}

			r.logger.InfoCtx(ctx, "found certificates needing renewal", log.Int("count", len(certificates)))

			for _, certificate := range certificates {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				r.logger.InfoCtx(ctx, "renewing certificate for hostname", log.String("hostname", certificate.Hostname))

				if err := r.renewCertificate(ctx, tx, certificate.ID); err != nil {
					r.logger.ErrorCtx(ctx, "cannot renew certificate", log.String("hostname", certificate.Hostname), log.Error(err))
				} else {
					r.logger.InfoCtx(ctx, "successfully renewed certificate", log.String("hostname", certificate.Hostname))
				}
			}

			return nil
		},
	)
}

func (r *Renewer) renewCertificate(ctx context.Context, tx pg.Tx, certificateID gid.GID) error {
	certificate := &coredata.Certificate{}
	if err := certificate.LoadByIDForUpdateSkipLocked(ctx, tx, coredata.NewNoScope(), certificateID); err != nil {
		if errors.Is(err, coredata.ErrResourceNotFound) {
			return nil
		}

		return fmt.Errorf("cannot lock certificate for renewal: %w", err)
	}

	if certificate.Status != coredata.CertificateStatusActive {
		r.logger.InfoCtx(
			ctx,
			"certificate status changed, skipping renewal",
			log.String("hostname", certificate.Hostname),
		)

		return nil
	}

	certificate.Status = coredata.CertificateStatusRenewing
	if err := certificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
		return fmt.Errorf("cannot update certificate status: %w", err)
	}

	return nil
}
