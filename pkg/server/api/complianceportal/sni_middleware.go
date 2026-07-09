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

package complianceportal

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.gearno.de/kit/httpserver"
	trust "go.probo.inc/probo/pkg/complianceportal/visitor"
	"go.probo.inc/probo/pkg/server/gqlutils"
)

func NewSNIMiddleware(trustSvc *trust.Service) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if r.TLS == nil {
				next.ServeHTTP(w, r)
				return
			}

			compliancePage, err := trustSvc.GetPortalByDomainName(ctx, r.TLS.ServerName)
			if err != nil {
				if errors.Is(err, trust.ErrPageNotFound) {
					next.ServeHTTP(w, r)
					return
				}

				httpserver.RenderJSON(
					w,
					http.StatusInternalServerError,
					&graphql.Response{
						Errors: gqlerror.List{
							gqlutils.Internal(ctx),
						},
					},
				)

				return
			}

			// Redirect secondary domains to the canonical host so a compliance
			// page is only ever served under a single origin. ACME HTTP-01
			// challenges are handled upstream and never reach this middleware.
			if !strings.HasPrefix(r.URL.Path, "/.well-known/") {
				canonicalHost, err := trustSvc.GetPortalEffectiveCanonicalHost(ctx, compliancePage.ID)
				if err != nil {
					httpserver.RenderJSON(
						w,
						http.StatusInternalServerError,
						&graphql.Response{
							Errors: gqlerror.List{
								gqlutils.Internal(ctx),
							},
						},
					)

					return
				}

				if canonicalHost != "" && canonicalHost != r.Host {
					target := &url.URL{
						Scheme:   "https",
						Host:     canonicalHost,
						Path:     r.URL.Path,
						RawQuery: r.URL.RawQuery,
					}

					http.Redirect(w, r, target.String(), http.StatusMovedPermanently)

					return
				}
			}

			baseURL := &url.URL{
				Host:   r.Host,
				Path:   r.URL.Path,
				Scheme: "https",
			}
			baseURLString := baseURL.String()

			ctx = context.WithValue(
				ctx,
				compliancePageBaseURLKey,
				&baseURLString,
			)
			r = r.WithContext(ctx)

			if compliancePage.Active {
				ctx = context.WithValue(ctx, compliancePageKey, compliancePage)
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
