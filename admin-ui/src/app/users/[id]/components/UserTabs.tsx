"use client";

import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Tab, TabList, TabPanel, Tabs } from "@/components/ui/tabs";
import { UserBasicDetails } from "./UserBasicDetails";
import { UserAddressDetails } from "./UserAddressDetails";
import { UserTransactionsTable } from "./UserTransactionsTable";
import { Button } from "@/components/ui/button";
import axios from "axios";
import { UserPlanSelector } from "./UserPlanSelector";

export function UserTabs({ id }: { id: number }) {
  const handleImpersonate = async () => {
    const { data } = await axios.get(
      `/api/proxy/main/admin/users/${id}/impersonate`
    );

    const url = `http://localhost:3333/impersonate/${data.code}`;
    window.open(url, "_blank", "noopener");
  };

  return (
    <Tabs aria-label="Recipe App">
      <TabList>
        <Tab id="g">General</Tab>
        <Tab id="k">KYC</Tab>
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
      <TabPanel id="k">
        Check the list of ingredients needed for your chosen recipes.
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
