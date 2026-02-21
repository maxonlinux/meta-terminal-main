"use client";

import { ChevronUpDownIcon } from "@heroicons/react/24/outline";
import {
  ArrowRightStartOnRectangleIcon,
  Cog6ToothIcon,
  HomeIcon,
  LifebuoyIcon,
  ShieldCheckIcon,
} from "@heroicons/react/24/solid";
import { Avatar } from "@/components/ui/avatar";
import { Link } from "@/components/ui/link";
import {
  Menu,
  MenuContent,
  MenuHeader,
  MenuItem,
  MenuSection,
  MenuSeparator,
  MenuTrigger,
} from "@/components/ui/menu";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarItem,
  SidebarLabel,
  SidebarRail,
  SidebarSection,
  SidebarSectionGroup,
} from "@/components/ui/sidebar";
import { Fingerprint, HashIcon, Users2, UserStar, Wallet } from "lucide-react";
import { usePathname } from "next/navigation";
import useSWR from "swr";
import axios from "axios";

const menu = [
  {
    href: "/users",
    label: "Users",
    countKey: "users",
    icon: Users2,
  },
  {
    href: "/wallets",
    label: "Wallets",
    countKey: "wallets",
    icon: Wallet,
  },
  {
    href: "/transactions",
    label: "Transactions",
    countKey: "transactions",
    icon: HashIcon,
  },
  {
    href: "/kyc",
    label: "KYCs",
    countKey: "kyc",
    icon: Fingerprint,
  },
];

export default function AppSidebar(
  props: React.ComponentProps<typeof Sidebar>
) {
  const pathname = usePathname();

  const { data } = useSWR(
    "/api/proxy/main/admin/pending-count",
    async (url) => {
      const { data } = await axios.get(url);
      return data;
    },
    {
      refreshInterval: 10_000,
    }
  );

  return (
    <Sidebar {...props}>
      <SidebarHeader>
        <Link href="/" className="flex items-center gap-x-2">
          <div className="flex items-center justify-center shrink-0 size-6 bg-primary rounded-sm">
            <UserStar className="size-4" />
          </div>
          {/* <Avatar
            isSquare
            size="sm"
            className="outline-hidden"
            src="https://design.intentui.com/logo?color=155DFC"
          /> */}
          <SidebarLabel className="font-medium">
            TERMINAL <span className="text-muted-fg">ADMIN</span>
          </SidebarLabel>
        </Link>
      </SidebarHeader>
      <SidebarContent>
        <SidebarSectionGroup>
          <SidebarSection label="Menu">
            {menu.map((link) => (
              <SidebarItem
                tooltip={link.label}
                key={link.href}
                href={link.href}
                badge={
                  data?.[link.countKey] > 0
                    ? `${data?.[link.countKey]} new`
                    : undefined
                }
                isCurrent={pathname === link.href}
              >
                <link.icon data-slot="icon" />
                <SidebarLabel>{link.label}</SidebarLabel>
              </SidebarItem>
            ))}
          </SidebarSection>
        </SidebarSectionGroup>
      </SidebarContent>

      <SidebarFooter className="flex flex-row justify-between gap-4 group-data-[state=collapsed]:flex-col">
        <Menu>
          <MenuTrigger
            className="flex w-full items-center justify-between"
            aria-label="Profile"
          >
            <div className="flex items-center gap-x-2">
              <Avatar
                className="size-8 *:size-8 group-data-[state=collapsed]:size-6 group-data-[state=collapsed]:*:size-6"
                isSquare
                src="https://intentui.com/images/avatar/cobain.jpg"
              />
              <div className="in-data-[collapsible=dock]:hidden text-sm">
                <SidebarLabel>Kurt Cobain</SidebarLabel>
                <span className="-mt-0.5 block text-muted-fg">
                  kurt@domain.com
                </span>
              </div>
            </div>
            <ChevronUpDownIcon data-slot="chevron" />
          </MenuTrigger>
          <MenuContent
            className="in-data-[sidebar-collapsible=collapsed]:min-w-56 min-w-(--trigger-width)"
            placement="bottom right"
          >
            <MenuSection>
              <MenuHeader separator>
                <span className="block">Kurt Cobain</span>
                <span className="font-normal text-muted-fg">@cobain</span>
              </MenuHeader>
            </MenuSection>

            <MenuItem href="#dashboard">
              <HomeIcon />
              Dashboard
            </MenuItem>
            <MenuItem href="#settings">
              <Cog6ToothIcon />
              Settings
            </MenuItem>
            <MenuItem href="#security">
              <ShieldCheckIcon />
              Security
            </MenuItem>
            <MenuSeparator />
            <MenuItem href="#contact">
              <LifebuoyIcon />
              Customer Support
            </MenuItem>
            <MenuSeparator />
            <MenuItem href="#logout">
              <ArrowRightStartOnRectangleIcon />
              Log out
            </MenuItem>
          </MenuContent>
        </Menu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
