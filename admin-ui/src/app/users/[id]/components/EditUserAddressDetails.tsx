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
import { TextField } from "@/components/ui/text-field";
import { Edit3 } from "lucide-react";

export function EditUserAddressDetails() {
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
              <SheetTitle>Update User Address</SheetTitle>
              <SheetDescription>Adjust user's address here.</SheetDescription>
            </SheetHeader>
            <SheetBody className="space-y-4">
              <TextField>
                <Label>Country</Label>
                <Input type="text" placeholder="USA" />
              </TextField>
              <TextField>
                <Label>City</Label>
                <Input type="text" placeholder="New York" />
              </TextField>
              <TextField>
                <Label>Address</Label>
                <Input type="text" placeholder="123 Main St" />
              </TextField>
              <TextField>
                <Label>Zip Code</Label>
                <Input type="text" placeholder="10001" />
              </TextField>
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
