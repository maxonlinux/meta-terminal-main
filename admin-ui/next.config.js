const normalizeBasePath = (value) => {
  if (!value) return "";
  const trimmed = value.trim();
  if (!trimmed || trimmed === "/") return "";
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
  return withSlash.endsWith("/") ? withSlash.slice(0, -1) : withSlash;
};

const basePath = normalizeBasePath(process.env.ADMIN_BASE_PATH);

const nextConfig = {
  reactCompiler: true,
  typescript: {
    ignoreBuildErrors: true,
  },
  basePath: basePath || undefined,
  assetPrefix: basePath || undefined,
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
