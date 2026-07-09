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

import { useTranslate } from "@probo/i18n";
import { Button, Card, IconPlusLarge } from "@probo/ui";
import { type PreloadedQuery, usePreloadedQuery } from "react-relay";
import { graphql } from "relay-runtime";

import type { CompliancePageDomainPageQuery } from "#/__generated__/core/CompliancePageDomainPageQuery.graphql";

import { CompliancePageDomainCard } from "./_components/CompliancePageDomainCard";
import { NewCompliancePageDomainDialog } from "./_components/NewCompliancePageDomainDialog";

export const compliancePageDomainPageQuery = graphql`
  query CompliancePageDomainPageQuery($organizationId: ID!) {
    organization: node(id: $organizationId) {
      __typename
      ... on Organization {
        canCreateCustomDomain: permission(action: "compliance-portal:custom-domain:create")
        compliancePage: trustCenter {
          id
          defaultDomain {
            id
            ...CompliancePageDomainCardFragment
          }
          customDomain {
            id
            ...CompliancePageDomainCardFragment
          }
        }
      }
    }
  }
`;

export function CompliancePageDomainPage(props: {
  queryRef: PreloadedQuery<CompliancePageDomainPageQuery>;
}) {
  const { queryRef } = props;

  const { __ } = useTranslate();

  const { organization } = usePreloadedQuery<CompliancePageDomainPageQuery>(
    compliancePageDomainPageQuery,
    queryRef,
  );
  if (organization.__typename !== "Organization") {
    throw new Error("invalid type for node");
  }

  const compliancePageId = organization.compliancePage?.id;
  const defaultDomain = organization.compliancePage?.defaultDomain;
  const customDomain = organization.compliancePage?.customDomain;

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-base font-medium">{__("Domains")}</h2>
        <p className="text-sm text-txt-tertiary">
          {__(
            "Your compliance page is always available on its default probopage.com subdomain. You can also serve it on one custom domain of your own.",
          )}
        </p>
      </div>

      {defaultDomain && compliancePageId && (
        <CompliancePageDomainCard
          fKey={defaultDomain}
          compliancePageId={compliancePageId}
        />
      )}

      {compliancePageId && customDomain
        ? (
            <CompliancePageDomainCard
              fKey={customDomain}
              compliancePageId={compliancePageId}
            />
          )
        : (
            <Card padded>
              <div className="text-center py-8">
                <h3 className="text-lg font-semibold mb-2">
                  {__("No custom domain configured")}
                </h3>
                <p className="text-txt-tertiary mb-4">
                  {__(
                    "Add your own domain to make your compliance page more professional",
                  )}
                </p>
                <div className="flex justify-center">
                  {compliancePageId && organization.canCreateCustomDomain && (
                    <NewCompliancePageDomainDialog compliancePageId={compliancePageId}>
                      <Button icon={IconPlusLarge}>{__("Add Domain")}</Button>
                    </NewCompliancePageDomainDialog>
                  )}
                </div>
              </div>
            </Card>
          )}
    </div>
  );
}
