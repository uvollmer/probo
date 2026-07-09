// Copyright (c) 2026 Probo Inc <hello@probo.com>.
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

package types

import (
	"go.probo.inc/probo/pkg/coredata"
)

// NewCustomDomain builds the MCP CustomDomain type. The TLS lifecycle now lives
// on the linked certificate; when cert is nil (certificate not yet created) the
// domain reports a pending SSL status.
func NewCustomDomain(d *coredata.CustomDomain, cert *coredata.Certificate) *CustomDomain {
	result := &CustomDomain{
		ID:             d.ID,
		OrganizationID: d.OrganizationID,
		Domain:         d.Domain,
		Managed:        d.Managed,
		SslStatus:      coredata.CustomDomainSSLStatusPending,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}

	if cert != nil {
		result.SslStatus = coredata.CustomDomainSSLStatus(cert.Status)
		result.SslExpiresAt = cert.SSLExpiresAt
	}

	return result
}
