"use client";

import { RotateCw } from "lucide-react";
import useSWR from "swr";
import { getUserAddress } from "@/api/admin";
import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  DescriptionDetails,
  DescriptionList,
  DescriptionTerm,
} from "@/components/ui/description-list";
import { Loader } from "@/components/ui/loader";
import { safeString } from "@/lib/utils";
import { UserAddress } from "@/types";
import { EditUserAddressDetails } from "./EditUserAddressDetails";

export function UserAddressDetails({ id }: { id: number }) {
  const { data, isLoading, isValidating, error, mutate } = useSWR(
    ["admin:user:address", id],
    () => getUserAddress(id),
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>User address</CardTitle>
        <CardDescription>
          The product details card is a great way to display information about a
          product.
        </CardDescription>
        <CardAction>
          <ButtonGroup>
            <Button intent="outline" onClick={() => mutate()}>
              {isValidating ? (
                <Loader variant="spin" />
              ) : (
                <RotateCw data-slot="icon" />
              )}
              Refresh
            </Button>
            {data && (
              <EditUserAddressDetails
                userId={id}
                address={data}
                onSaved={() => mutate()}
              />
            )}
          </ButtonGroup>
        </CardAction>
      </CardHeader>
      <CardContent>
        {data && (
          <DescriptionList>
            <DescriptionTerm>Country</DescriptionTerm>
            <DescriptionDetails>{safeString(data.country)}</DescriptionDetails>
            <DescriptionTerm>City</DescriptionTerm>
            <DescriptionDetails>{safeString(data.city)}</DescriptionDetails>
            <DescriptionTerm>Address</DescriptionTerm>
            <DescriptionDetails>{safeString(data.address)}</DescriptionDetails>
            <DescriptionTerm>Zip</DescriptionTerm>
            <DescriptionDetails>{safeString(data.zip)}</DescriptionDetails>
          </DescriptionList>
        )}

        {isLoading && (
          <div className="flex items-center justify-center w-full">
            <Loader variant="spin" />
          </div>
        )}
        {error && <div>Error: {error.message}</div>}
      </CardContent>
    </Card>
  );
}
