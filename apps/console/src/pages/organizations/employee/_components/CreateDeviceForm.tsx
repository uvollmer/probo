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

import { formatError, type GraphQLError } from "@probo/helpers";
import { useTranslate } from "@probo/i18n";
import { Button, useToast } from "@probo/ui";
import { useState } from "react";
import { useMutation } from "react-relay";
import { graphql } from "relay-runtime";

import type { CreateDeviceFormMutation } from "#/__generated__/core/CreateDeviceFormMutation.graphql";
import { useOrganizationId } from "#/hooks/useOrganizationId";

import { EnrollmentInstructions } from "./EnrollmentInstructions";

const createDeviceMutation = graphql`
  mutation CreateDeviceFormMutation($input: CreateDeviceInput!) {
    createDevice(input: $input) {
      apiKey
      device {
        id
      }
    }
  }
`;

export function CreateDeviceForm() {
  const { __ } = useTranslate();
  const { toast } = useToast();

  const [apiKey, setApiKey] = useState<string | null>(null);

  const organizationId = useOrganizationId();
  const [createDevice, isCreating] = useMutation<CreateDeviceFormMutation>(
    createDeviceMutation,
  );

  const handleCreate = () => {
    createDevice({
      variables: {
        input: {
          organizationId,
        },
      },
      onCompleted(response, errors) {
        if (errors?.length) {
          toast({
            title: __("Error"),
            description: errors[0].message,
            variant: "error",
          });
          return;
        }

        setApiKey(response.createDevice.apiKey);
        toast({
          title: __("Success"),
          description: __(
            "Device created. Copy the API key now — it will not be shown again.",
          ),
          variant: "success",
        });
      },
      onError(error) {
        toast({
          title: __("Error"),
          description: formatError(
            __("Failed to create device"),
            error as GraphQLError,
          ),
          variant: "error",
        });
      },
    });
  };

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <Button onClick={handleCreate} disabled={isCreating}>
          {__("Create device and generate API key")}
        </Button>
      </div>
      {apiKey && <EnrollmentInstructions apiKey={apiKey} />}
    </div>
  );
}
