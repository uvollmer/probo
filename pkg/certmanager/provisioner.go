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
	"strings"
	"time"

	"codeberg.org/miekg/dns"
	"go.gearno.de/kit/log"
	"go.gearno.de/kit/pg"
	"go.probo.inc/probo/pkg/coredata"
	"go.probo.inc/probo/pkg/crypto/cipher"
	"go.probo.inc/probo/pkg/gid"
)

type (
	Provisioner struct {
		pg              *pg.Client
		acmeService     *ACMEService
		encryptionKey   cipher.EncryptionKey
		cnameTarget     string
		caaIssuerDomain string
		interval        time.Duration
		resolverAddr    string
		logger          *log.Logger
	}
)

const (
	maxRetries = 3
)

func NewProvisioner(
	pg *pg.Client,
	acmeService *ACMEService,
	encryptionKey cipher.EncryptionKey,
	cnameTarget string,
	caaIssuerDomain string,
	interval time.Duration,
	resolverAddr string,
	logger *log.Logger,
) *Provisioner {
	return &Provisioner{
		pg:              pg,
		acmeService:     acmeService,
		encryptionKey:   encryptionKey,
		cnameTarget:     cnameTarget,
		caaIssuerDomain: caaIssuerDomain,
		interval:        interval,
		resolverAddr:    resolverAddr,
		logger:          logger.Named("certmanager.provisioner"),
	}
}

func (p *Provisioner) Run(ctx context.Context) error {
	p.logger.InfoCtx(ctx, "certificate provisioner starting", log.Duration("interval", p.interval))

	if err := p.checkPendingCertificates(ctx); err != nil {
		p.logger.ErrorCtx(ctx, "initial check failed", log.Error(err))
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.InfoCtx(ctx, "certificate provisioner shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := p.checkPendingCertificates(ctx); err != nil {
				p.logger.ErrorCtx(ctx, "periodic check failed", log.Error(err))
			}
		}
	}
}

func (p *Provisioner) checkDNSConfiguration(hostname string) error {
	customerFQDN := hostname
	if !strings.HasSuffix(customerFQDN, ".") {
		customerFQDN = customerFQDN + "."
	}

	expectedFQDN := p.cnameTarget
	if !strings.HasSuffix(expectedFQDN, ".") {
		expectedFQDN = expectedFQDN + "."
	}

	msg := &dns.Msg{MsgHeader: dns.MsgHeader{ID: dns.ID(), RecursionDesired: true}}
	msg.Question = []dns.RR{&dns.CNAME{Hdr: dns.Header{Name: customerFQDN, Class: dns.ClassINET}}}

	client := dns.NewClient()

	resp, _, err := client.Exchange(context.Background(), msg, "udp", p.resolverAddr)
	if err != nil {
		return fmt.Errorf("cannot exchange dns message: %w", err)
	}

	if len(resp.Answer) == 0 {
		return fmt.Errorf("no cname records found for domain %q", hostname)
	}

	if len(resp.Answer) > 1 {
		return fmt.Errorf("multiple cname records found for domain %q", hostname)
	}

	resolvedRecord, ok := resp.Answer[0].(*dns.CNAME)
	if !ok {
		return fmt.Errorf("first answer is not a cname record for domain %q", hostname)
	}

	if !strings.EqualFold(expectedFQDN, resolvedRecord.Target) {
		return fmt.Errorf(
			"cname target mismatch: domain %q resolves to %q, expected %q",
			hostname,
			resolvedRecord.Target,
			expectedFQDN,
		)
	}

	return nil
}

func (p *Provisioner) checkCAARecords(hostname string) error {
	fqdn := hostname
	if !strings.HasSuffix(fqdn, ".") {
		fqdn = fqdn + "."
	}

	msg := &dns.Msg{MsgHeader: dns.MsgHeader{ID: dns.ID(), RecursionDesired: true}}
	msg.Question = []dns.RR{&dns.CAA{Hdr: dns.Header{Name: fqdn, Class: dns.ClassINET}}}

	client := dns.NewClient()

	resp, _, err := client.Exchange(
		context.Background(),
		msg,
		"udp",
		p.resolverAddr,
	)
	if err != nil {
		return fmt.Errorf("cannot exchange dns message for caa records: %w", err)
	}

	var caaRecords []*dns.CAA

	for _, rr := range resp.Answer {
		if caa, ok := rr.(*dns.CAA); ok {
			caaRecords = append(caaRecords, caa)
		}
	}

	if len(caaRecords) == 0 {
		return nil
	}

	for _, caa := range caaRecords {
		if caa.Tag == "issue" {
			issuer, _, _ := strings.Cut(caa.Value, ";")
			if strings.EqualFold(strings.TrimSpace(issuer), p.caaIssuerDomain) {
				return nil
			}
		}
	}

	return fmt.Errorf(
		"caa records for domain %q do not permit issuance by %q",
		hostname,
		p.caaIssuerDomain,
	)
}

func (p *Provisioner) checkPendingCertificates(ctx context.Context) error {
	err := p.pg.WithTx(
		ctx,
		func(ctx context.Context, tx pg.Tx) error {
			if err := p.handleStaleProvisioningAttempts(ctx, tx); err != nil {
				return fmt.Errorf("cannot handle stale provisioning attempts: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("cannot handle stale provisioning attempts: %w", err)
	}

	err = p.pg.WithTx(
		ctx,
		func(ctx context.Context, tx pg.Tx) error {
			var certificates coredata.Certificates
			if err := certificates.ListWithPendingHTTPChallenges(ctx, tx, coredata.NewNoScope()); err != nil {
				return fmt.Errorf("cannot load certificates with pending challenges: %w", err)
			}

			if len(certificates) == 0 {
				return nil
			}

			p.logger.InfoCtx(ctx, "found certificates needing SSL provisioning", log.Int("count", len(certificates)))

			for _, certificate := range certificates {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				if err := p.provisionCertificate(ctx, tx, certificate.ID); err != nil {
					p.logger.ErrorCtx(
						ctx,
						"cannot provision certificate",
						log.String("hostname", certificate.Hostname),
						log.Error(err),
					)
				}
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("cannot provision certificates: %w", err)
	}

	return nil
}

func (p *Provisioner) handleStaleProvisioningAttempts(ctx context.Context, tx pg.Tx) error {
	var certificates coredata.Certificates
	if err := certificates.ListStaleProvisioning(ctx, tx, coredata.NewNoScope()); err != nil {
		return fmt.Errorf("cannot load stale provisioning certificates: %w", err)
	}

	if len(certificates) == 0 {
		return nil
	}

	p.logger.InfoCtx(ctx, "found stale provisioning attempts to reset", log.Int("count", len(certificates)))

	for _, certificate := range certificates {
		if err := p.resetStaleCertificate(ctx, tx, certificate); err != nil {
			p.logger.ErrorCtx(
				ctx,
				"cannot reset stale certificate",
				log.String("hostname", certificate.Hostname),
				log.Error(err),
			)
		}
	}

	return nil
}

func (p *Provisioner) resetStaleCertificate(
	ctx context.Context,
	tx pg.Tx,
	certificate *coredata.Certificate,
) error {
	fullCertificate := &coredata.Certificate{}
	if err := fullCertificate.LoadByIDForUpdateSkipLocked(ctx, tx, coredata.NewNoScope(), certificate.ID); err != nil {
		if errors.Is(err, coredata.ErrResourceNotFound) {
			return nil
		}

		return fmt.Errorf("cannot load stale certificate for update: %w", err)
	}

	staleDuration := time.Since(fullCertificate.UpdatedAt)

	p.logger.InfoCtx(
		ctx,
		"resetting stale certificate",
		log.String("hostname", fullCertificate.Hostname),
		log.String("status", string(fullCertificate.Status)),
		log.Duration("stale_duration", staleDuration),
		log.Int("retry_count", fullCertificate.SSLRetryCount),
	)

	fullCertificate.HTTPChallengeToken = nil
	fullCertificate.HTTPChallengeKeyAuth = nil
	fullCertificate.HTTPChallengeURL = nil
	fullCertificate.HTTPOrderURL = nil
	fullCertificate.ProvisioningError = nil
	fullCertificate.Status = coredata.CertificateStatusPending

	if fullCertificate.SSLLastAttemptAt != nil && time.Since(*fullCertificate.SSLLastAttemptAt) > 24*time.Hour {
		p.logger.InfoCtx(
			ctx,
			"resetting retry count due to old last attempt",
			log.String("hostname", fullCertificate.Hostname),
			log.Time("last_attempt", *fullCertificate.SSLLastAttemptAt),
		)
		fullCertificate.SSLRetryCount = 0
		fullCertificate.SSLLastAttemptAt = nil
	}

	if err := fullCertificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
		return fmt.Errorf("cannot update stale certificate: %w", err)
	}

	return nil
}

func (p *Provisioner) provisionCertificate(
	ctx context.Context,
	tx pg.Tx,
	certificateID gid.GID,
) error {
	certificate := &coredata.Certificate{}
	if err := certificate.LoadByIDForUpdateSkipLocked(ctx, tx, coredata.NewNoScope(), certificateID); err != nil {
		if errors.Is(err, coredata.ErrResourceNotFound) {
			return nil
		}

		return fmt.Errorf("cannot load by id for update %q certificate: %w", certificateID, err)
	}

	if certificate.Status == coredata.CertificateStatusPending || certificate.Status == coredata.CertificateStatusRenewing {
		if err := p.checkDNSConfiguration(certificate.Hostname); err != nil {
			p.logger.WarnCtx(
				ctx,
				"dns configuration check failed",
				log.String("hostname", certificate.Hostname),
				log.Error(err),
			)

			errMsg := err.Error()

			certificate.ProvisioningError = &errMsg
			if err := certificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
				return fmt.Errorf("cannot update certificate with provisioning error: %w", err)
			}

			return nil
		}

		if err := p.checkCAARecords(certificate.Hostname); err != nil {
			p.logger.WarnCtx(
				ctx,
				"caa record check failed",
				log.String("hostname", certificate.Hostname),
				log.Error(err),
			)

			errMsg := err.Error()

			certificate.ProvisioningError = &errMsg
			if err := certificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
				return fmt.Errorf("cannot update certificate with provisioning error: %w", err)
			}

			return nil
		}

		certificate.ProvisioningError = nil
		if err := certificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
			return fmt.Errorf("cannot clear provisioning error: %w", err)
		}

		p.logger.InfoCtx(ctx, "DNS configuration verified, initiating HTTP challenge for hostname", log.String("hostname", certificate.Hostname))

		challenge, err := p.acmeService.GetHTTPChallenge(ctx, certificate.Hostname)
		if err != nil {
			p.logger.ErrorCtx(
				ctx,
				"cannot get HTTP challenge",
				log.String("hostname", certificate.Hostname),
				log.Error(err),
			)

			return err
		}

		certificate.HTTPChallengeToken = &challenge.Token
		certificate.HTTPChallengeKeyAuth = &challenge.KeyAuth
		certificate.HTTPChallengeURL = &challenge.URL
		certificate.HTTPOrderURL = &challenge.OrderURL
		certificate.Status = coredata.CertificateStatusProvisioning

		if err := certificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
			return fmt.Errorf("cannot update certificate with challenge: %w", err)
		}

		p.logger.InfoCtx(
			ctx,
			"HTTP challenge initiated, will complete in next cycle",
			log.String("hostname", certificate.Hostname),
			log.String("token", challenge.Token),
		)

		return nil
	}

	challenge := &HTTPChallenge{
		Domain:   certificate.Hostname,
		Token:    *certificate.HTTPChallengeToken,
		KeyAuth:  *certificate.HTTPChallengeKeyAuth,
		URL:      *certificate.HTTPChallengeURL,
		OrderURL: *certificate.HTTPOrderURL,
	}

	cert, err := p.acmeService.CompleteHTTPChallenge(ctx, challenge)
	if err != nil {
		p.logger.WarnCtx(
			ctx,
			"cannot complete HTTP challenge",
			log.String("hostname", certificate.Hostname),
			log.Int("retry_count", certificate.SSLRetryCount),
			log.Error(err),
		)

		errMsg := err.Error()
		certificate.ProvisioningError = &errMsg
		certificate.SSLRetryCount = certificate.SSLRetryCount + 1
		now := time.Now()
		certificate.SSLLastAttemptAt = &now

		// Clear challenge data and reset to pending so the next attempt
		// creates a fresh ACME order. Once a challenge fails validation,
		// Let's Encrypt marks it as invalid and retrying the same
		// challenge always fails with "authorization must be pending".
		certificate.HTTPChallengeToken = nil
		certificate.HTTPChallengeKeyAuth = nil
		certificate.HTTPChallengeURL = nil
		certificate.HTTPOrderURL = nil

		if certificate.SSLRetryCount >= maxRetries {
			p.logger.ErrorCtx(
				ctx,
				"certificate has exceeded max retry attempts, marking as failed",
				log.String("hostname", certificate.Hostname),
				log.Int("retry_count", certificate.SSLRetryCount),
			)

			certificate.Status = coredata.CertificateStatusFailed
		} else {
			certificate.Status = coredata.CertificateStatusPending
		}

		if err := certificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
			return fmt.Errorf("cannot update certificate: %w", err)
		}

		return nil
	}

	p.logger.InfoCtx(
		ctx,
		"certificate obtained successfully",
		log.String("hostname", certificate.Hostname),
		log.Time("expires_at", cert.ExpiresAt),
	)

	certificate.ProvisioningError = nil

	certificate.SSLCertificatePEM = cert.CertPEM
	if err := certificate.EncryptPrivateKey(cert.KeyPEM, p.encryptionKey); err != nil {
		return fmt.Errorf("cannot encrypt private key: %w", err)
	}

	chainStr := string(cert.ChainPEM)
	certificate.SSLCertificateChain = &chainStr
	certificate.SSLExpiresAt = &cert.ExpiresAt
	certificate.Status = coredata.CertificateStatusActive

	certificate.SSLRetryCount = 0
	certificate.SSLLastAttemptAt = nil

	certificate.HTTPChallengeToken = nil
	certificate.HTTPChallengeKeyAuth = nil
	certificate.HTTPChallengeURL = nil
	certificate.HTTPOrderURL = nil

	if err := certificate.Update(ctx, tx, coredata.NewNoScope()); err != nil {
		return fmt.Errorf("cannot update certificate: %w", err)
	}

	cache := &coredata.CachedCertificate{
		Domain:           certificate.Hostname,
		CertificatePEM:   string(cert.CertPEM),
		PrivateKeyPEM:    string(cert.KeyPEM),
		CertificateChain: &chainStr,
		ExpiresAt:        cert.ExpiresAt,
		CachedAt:         time.Now(),
		CertificateID:    certificate.ID,
	}

	if err := cache.Upsert(ctx, tx); err != nil {
		p.logger.ErrorCtx(
			ctx,
			"cannot update certificate cache",
			log.String("hostname", certificate.Hostname),
			log.Error(err),
		)
	}

	return nil
}
