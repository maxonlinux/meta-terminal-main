const nextConfig = {
  reactCompiler: true,
  typescript: {
    ignoreBuildErrors: true,
  },
  basePath: process.env.ADMIN_BASE_PATH || undefined,
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

module.exports = nextConfig;
