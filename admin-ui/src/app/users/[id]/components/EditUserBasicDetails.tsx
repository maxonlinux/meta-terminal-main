"use client";

import { Button } from "@/components/ui/button";
import { Checkbox, CheckboxLabel } from "@/components/ui/checkbox";
import { Description, Label } from "@/components/ui/field";
import { Input, InputGroup } from "@/components/ui/input";
import {
  Sheet,
  SheetBody,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Switch, SwitchLabel } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { TextField } from "@/components/ui/text-field";
import { Edit3, Plus } from "lucide-react";

export function EditUserBasicDetails() {
  return (
    <Sheet>
      <Button intent="outline">
        <Edit3 data-slot="icon" />
        Edit
      </Button>
      <SheetContent>
        {({ close }) => (
          <>
            <SheetHeader>
              <SheetTitle>Update User Settings</SheetTitle>
              <SheetDescription>
                Adjust user's configurations here.
              </SheetDescription>
            </SheetHeader>
            <SheetBody className="space-y-4">
              <div className="flex items-center gap-2">
                <TextField>
                  <Label>First name</Label>
                  <Input type="text" placeholder="John" />
                </TextField>
                <TextField>
                  <Label>Last name</Label>
                  <Input type="text" placeholder="Smith" />
                </TextField>
              </div>
              <TextField>
                <Label>Email</Label>
                <Input type="email" placeholder="Enter email address" />
              </TextField>
              <TextField>
                <Label>Username</Label>
                <Input type="text" placeholder="Enter username" />
              </TextField>
              <TextField>
                <Label>Phone</Label>
                <InputGroup>
                  <Plus data-slot="icon" />
                  <Input type="text" placeholder="1 123 456 78 90" />
                </InputGroup>
              </TextField>

              <Separator />

              <Switch>Active</Switch>
            </SheetBody>
            <SheetFooter>
              <Button onPress={close} intent="primary" type="submit">
                Save
              </Button>
              <SheetClose>Cancel</SheetClose>
            </SheetFooter>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
