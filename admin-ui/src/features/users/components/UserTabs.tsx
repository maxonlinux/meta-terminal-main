"use client";

import { getUserImpersonation } from "@/api/admin";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Tab, TabList, TabPanel, Tabs } from "@/components/ui/tabs";
import { UserAddressDetails } from "./UserAddressDetails";
import { UserBasicDetails } from "./UserBasicDetails";
import { UserKyc } from "./UserKyc";
import { UserPlanSelector } from "./UserPlanSelector";
import { UserTransactionsTable } from "./UserTransactionsTable";
import { UserWallets } from "./UserWallets";

export function UserTabs({ id }: { id: string }) {
  const handleImpersonate = async () => {
    const data = await getUserImpersonation(id);
    if (!data?.code) return;
    const frontendBase = process.env.NEXT_PUBLIC_FRONTEND_URL?.replace(
      /\/$/,
      "",
    );
    const impersonatePath = `/impersonate/${data.code}`;
    const url = frontendBase
      ? `${frontendBase}${impersonatePath}`
      : impersonatePath;
    window.open(url, "_blank", "noopener");
  };

  return (
    <Tabs aria-label="Recipe App">
      <TabList>
        <Tab id="g">General</Tab>
        <Tab id="k">KYC</Tab>
        <Tab id="w">Wallets</Tab>
        <Tab id="o">Orders</Tab>
        <Tab id="t">Transactions</Tab>
      </TabList>
      <TabPanel id="g" className="flex flex-col gap-4">
        <Card>
          <CardHeader>
            <CardTitle>Impersonate</CardTitle>
            <CardDescription>
              Click button to impersonate this user (to log in as this user).
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button intent="primary" onClick={handleImpersonate}>
              Impersonate
            </Button>
          </CardContent>
        </Card>

        <UserPlanSelector id={id} />
        <UserBasicDetails id={id} />
        <UserAddressDetails id={id} />
      </TabPanel>
      <TabPanel id="k" className="flex flex-col gap-4">
        <UserKyc id={id} />
      </TabPanel>
      <TabPanel id="w" className="flex flex-col gap-4">
        <UserWallets id={id} />
      </TabPanel>
      <TabPanel id="o">
        Check the list of ingredients needed for your chosen recipes.
      </TabPanel>
      <TabPanel id="t">
        <UserTransactionsTable id={id} />
      </TabPanel>
    </Tabs>
  );
}
