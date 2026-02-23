import {
  ArrowUpRight,
  Fingerprint,
  HashIcon,
  Users2,
  Wallet,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Link } from "@/components/ui/link";

const links = [
  {
    href: "/users",
    title: "Users",
    description: "Manage user profiles, plans, and access.",
    icon: Users2,
    accent: "bg-sky-500/10 text-sky-300",
  },
  {
    href: "/wallets",
    title: "Wallets",
    description: "Review balances, locks, and wallet health.",
    icon: Wallet,
    accent: "bg-emerald-500/10 text-emerald-300",
  },
  {
    href: "/transactions",
    title: "Transactions",
    description: "Approve funding, audit activity, and resolve issues.",
    icon: HashIcon,
    accent: "bg-amber-500/10 text-amber-300",
  },
  {
    href: "/kyc",
    title: "KYC",
    description: "Verify identities and manage compliance flows.",
    icon: Fingerprint,
    accent: "bg-violet-500/10 text-violet-300",
  },
];

export default function Home() {
  return (
    <div className="flex flex-col gap-6">
      <div className="rounded-2xl border border-border bg-linear-to-br from-neutral-900 via-neutral-900 to-indigo-950/60 p-6 text-white shadow-lg">
        <div className="flex flex-col gap-2">
          <p className="text-xs uppercase tracking-[0.2em] text-white/50">
            Terminal Admin
          </p>
          <h1 className="text-2xl font-semibold">Operations control center</h1>
          <p className="text-sm text-white/60 max-w-2xl">
            Review users, funding, wallets, and compliance in one place. Jump
            into the modules below to approve workflows and keep the platform
            healthy.
          </p>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {links.map((item) => (
          <Card key={item.href} className="group border-border">
            <CardHeader className="flex flex-row items-start justify-between">
              <div>
                <CardTitle className="text-lg">{item.title}</CardTitle>
                <p className="text-sm text-muted-fg mt-1">{item.description}</p>
              </div>
              <div className={`rounded-sm p-2 ${item.accent}`}>
                <item.icon className="size-4" />
              </div>
            </CardHeader>
            <CardContent>
              <Link
                href={item.href}
                className="inline-flex items-center gap-2 text-sm text-primary"
              >
                Open module
                <ArrowUpRight className="size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5" />
              </Link>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
