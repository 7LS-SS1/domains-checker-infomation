import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  // Traces only the node_modules actually needed at runtime into
  // .next/standalone, so the production Docker image (web/Dockerfile)
  // doesn't have to ship the full node_modules tree or the source repo.
  output: "standalone",
  typescript: {
    ignoreBuildErrors: false,
  },
  // Playwright drives the dev server over 127.0.0.1 (see playwright.config.ts);
  // without this, Next's dev-only cross-origin asset guard blocks HMR/static
  // chunk requests from that host. Irrelevant to production builds.
  allowedDevOrigins: ["127.0.0.1", "localhost"],
};

export default withNextIntl(nextConfig);
