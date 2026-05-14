import type { Metadata } from "next";
import { IBM_Plex_Sans } from "next/font/google";
import "./globals.css";
import { Providers } from "@/components/providers";
import { Toast } from "@/components/ui/toast";

const ibmPlexSans = IBM_Plex_Sans({
  weight: ["200", "300", "400", "500", "600", "700"],
  variable: "--font-ibm-plex-sans",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: {
    default: "Main Admin",
    template: "%s | Main Admin",
  },
  description: "Meta Terminal Main Admin",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${ibmPlexSans.variable} antialiased dark`}>
        <Providers>
          <Toast />
          {children}
        </Providers>
      </body>
    </html>
  );
}
