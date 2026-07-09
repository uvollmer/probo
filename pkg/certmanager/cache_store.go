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
	"fmt"
	"time"

	"go.gearno.de/kit/log"
	"go.gearno.de/kit/pg"
	"go.probo.inc/probo/pkg/coredata"
	"go.probo.inc/probo/pkg/crypto/cipher"
)

type (
	CacheStore struct {
		pg            *pg.Client
		encryptionKey cipher.EncryptionKey
		logger        *log.Logger
	}
)

func NewCacheStore(
	pg *pg.Client,
	encryptionKey cipher.EncryptionKey,
	logger *log.Logger,
) *CacheStore {
	return &CacheStore{
		pg:            pg,
		encryptionKey: encryptionKey,
		logger:        logger.Named("certmanager.cache-store"),
	}
}

func (w *CacheStore) WarmCache(ctx context.Context) error {
	w.logger.InfoCtx(ctx, "warming certificate cache")

	startTime := time.Now()

	err := w.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			certificates := coredata.Certificates{}
			if err := certificates.LoadActive(ctx, conn, coredata.NewNoScope()); err != nil {
				return fmt.Errorf("cannot load active certificates: %w", err)
			}

			if len(certificates) == 0 {
				w.logger.InfoCtx(ctx, "no active certificates to warm")
				return nil
			}

			w.logger.InfoCtx(ctx, "found active certificates to cache", log.Int("count", len(certificates)))

			successCount := 0

			for _, certificate := range certificates {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				if err := w.warmCertificate(ctx, conn, certificate); err != nil {
					w.logger.ErrorCtx(ctx, "cannot warm certificate cache for hostname", log.String("hostname", certificate.Hostname), log.Error(err))
				} else {
					successCount++
				}
			}

			w.logger.InfoCtx(ctx, "successfully warmed cache", log.Int("success_count", successCount), log.Int("total_count", len(certificates)))

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("cannot warm certificate cache: %w", err)
	}

	w.logger.InfoCtx(ctx, "certificate cache warming completed", log.Duration("duration", time.Since(startTime)))

	return nil
}

func (w *CacheStore) warmCertificate(ctx context.Context, conn pg.Querier, certificate *coredata.Certificate) error {
	var loadedCertificate coredata.Certificate
	if err := loadedCertificate.LoadByID(ctx, conn, coredata.NewNoScope(), certificate.ID); err != nil {
		return fmt.Errorf("cannot load certificate with decrypted values: %w", err)
	}

	if err := loadedCertificate.ParseCertificate(w.encryptionKey); err != nil {
		return fmt.Errorf("cannot parse certificate: %w", err)
	}

	if len(loadedCertificate.SSLCertificatePEM) == 0 {
		return fmt.Errorf("certificate has no certificate PEM")
	}

	privateKeyPEM, err := loadedCertificate.DecryptPrivateKey(w.encryptionKey)
	if err != nil {
		return fmt.Errorf("cannot decrypt private key: %w", err)
	}

	if len(privateKeyPEM) == 0 {
		return fmt.Errorf("certificate has no private key PEM")
	}

	if loadedCertificate.SSLExpiresAt == nil {
		return fmt.Errorf("certificate has no expiry date")
	}

	if time.Now().After(*loadedCertificate.SSLExpiresAt) {
		return fmt.Errorf("certificate has expired")
	}

	cache := &coredata.CachedCertificate{
		Domain:           loadedCertificate.Hostname,
		CertificatePEM:   string(loadedCertificate.SSLCertificatePEM),
		PrivateKeyPEM:    string(privateKeyPEM),
		CertificateChain: loadedCertificate.SSLCertificateChain,
		ExpiresAt:        *loadedCertificate.SSLExpiresAt,
		CachedAt:         time.Now(),
		CertificateID:    loadedCertificate.ID,
	}

	if err := cache.Upsert(ctx, conn); err != nil {
		return fmt.Errorf("cannot upsert cache entry: %w", err)
	}

	return nil
}
