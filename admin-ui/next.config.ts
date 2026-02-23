import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  /* config options here */
  reactCompiler: true,
  typescript: {
    ignoreBuildErrors: true,
  },
  eslint: {
    ignoreDuringBuilds: true,
  },
  async rewrites() {
    const coreUrl = process.env.CORE_URL;
    if (!coreUrl) {
      return [];
    }

    return [
      {
        source: "/proxy/main/:path*",
        destination: `${coreUrl}/:path*`,
      },
    ];
  },
};

export default nextConfig;
