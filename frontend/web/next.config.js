/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://sw_api:4000/api/v1/:path*", // TODO: config?
      },
    ]
  },
}

export default nextConfig
