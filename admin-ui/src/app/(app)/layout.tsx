import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import AppSidebar from "@/features/shell/components/AppSidebar";
import AppSidebarNav from "@/features/shell/components/AppSidebarNav";

export default function AppLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <SidebarProvider>
      <AppSidebar collapsible="dock" />
      <SidebarInset>
        <AppSidebarNav />
        <main className="p-4 lg:p-6">{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
}
