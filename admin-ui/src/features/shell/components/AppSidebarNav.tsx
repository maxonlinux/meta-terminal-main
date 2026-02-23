"use client";

import {
  ArrowRightOnRectangleIcon,
  Cog6ToothIcon,
  CommandLineIcon,
  Squares2X2Icon,
} from "@heroicons/react/24/outline";
import { usePathname, useRouter } from "next/navigation";
import { Avatar } from "@/components/ui/avatar";
import { Breadcrumbs, BreadcrumbsItem } from "@/components/ui/breadcrumbs";
import {
  Menu,
  MenuContent,
  MenuHeader,
  MenuItem,
  MenuLabel,
  MenuSection,
  MenuSeparator,
  MenuTrigger,
} from "@/components/ui/menu";
import { SidebarNav, SidebarTrigger } from "@/components/ui/sidebar";
import { adminLogout } from "@/api/auth";

function DynamicBreadcrumbs() {
  const pathname = usePathname();
  const parts = pathname.split("/").filter(Boolean);

  const crumbs = parts.map((part, i) => ({
    label: part.charAt(0).toUpperCase() + part.slice(1),
    href: "/" + parts.slice(0, i + 1).join("/"),
  }));

  return (
    <Breadcrumbs className="hidden md:flex">
      {crumbs.map((c, idx) => (
        <BreadcrumbsItem
          key={c.href}
          href={idx === crumbs.length - 1 ? undefined : c.href} // last is plain text
        >
          {c.label}
        </BreadcrumbsItem>
      ))}
    </Breadcrumbs>
  );
}

function UserMenu() {
  const router = useRouter();

  const handleLogout = async () => {
    await adminLogout();
    router.push("/login");
  };

  return (
    <Menu>
      <MenuTrigger className="ml-auto md:hidden" aria-label="Open Menu">
        <Avatar
          isSquare
          alt="kurt cobain"
          src="https://intentui.com/images/avatar/cobain.jpg"
        />
      </MenuTrigger>
      <MenuContent popover={{ placement: "bottom end" }} className="min-w-64">
        <MenuSection>
          <MenuHeader separator>
            <span className="block">Kurt Cobain</span>
            <span className="font-normal text-muted-fg">@cobain</span>
          </MenuHeader>
        </MenuSection>
        <MenuItem href="#dashboard">
          <Squares2X2Icon />
          <MenuLabel>Dashboard</MenuLabel>
        </MenuItem>
        <MenuItem href="#settings">
          <Cog6ToothIcon />
          <MenuLabel>Settings</MenuLabel>
        </MenuItem>
        <MenuSeparator />
        <MenuItem>
          <CommandLineIcon />
          <MenuLabel>Command Menu</MenuLabel>
        </MenuItem>
        <MenuSeparator />
        <MenuItem href="#contact-s">
          <MenuLabel>Contact Support</MenuLabel>
        </MenuItem>
        <MenuSeparator />
        <MenuItem onAction={handleLogout}>
          <ArrowRightOnRectangleIcon />
          <MenuLabel>Log out</MenuLabel>
        </MenuItem>
      </MenuContent>
    </Menu>
  );
}

export default function AppSidebarNav() {
  return (
    <SidebarNav>
      <span className="flex items-center gap-x-4">
        <SidebarTrigger />
        <DynamicBreadcrumbs />
      </span>
      <UserMenu />
    </SidebarNav>
  );
}
