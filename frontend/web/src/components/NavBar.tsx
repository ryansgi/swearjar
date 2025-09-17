"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import * as React from "react"

function GitHubMark(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" {...props}>
      <path
        fill="currentColor"
        d="M8 .2a8 8 0 0 0-2.53 15.6c.4.07.55-.17.55-.38v-1.35C4.5 14.5 4 13.2 4 13.2c-.36-.92-.88-1.16-.88-1.16-.72-.49.05-.48.05-.48.8.06 1.23.84 1.23.84.71 1.22 1.87.87 2.32.66.07-.52.28-.87.5-1.07-2-.23-4.1-1-4.1-4.37 0-.97.35-1.76.92-2.38-.09-.23-.4-1.17.09-2.44 0 0 .76-.24 2.49.9a8.6 8.6 0 0 1 4.54 0c1.73-1.14 2.49-.9 2.49-.9.49 1.27.18 2.21.09 2.44.57.62.92 1.41.92 2.38 0 3.38-2.1 4.13-4.11 4.36.29.25.54.74.54 1.49v2.21c0 .21.14.46.55.38A8 8 0 0 0 8 .2Z"
      />
    </svg>
  )
}

const links = [
  { href: "/", label: "Home" },
  { href: "/explore", label: "Explore" },
  { href: "/about", label: "About" },
]

export default function NavBar() {
  const pathname = usePathname()
  const [elevated, setElevated] = React.useState(false)

  React.useEffect(() => {
    const onScroll = () => setElevated(window.scrollY > 4)
    onScroll()
    window.addEventListener("scroll", onScroll, { passive: true })
    return () => window.removeEventListener("scroll", onScroll)
  }, [])

  return (
    <nav
      className={[
        "fixed inset-x-0 top-0 z-40",
        "backdrop-blur-md",
        elevated
          ? "border-b border-white/10 bg-black/30 shadow-[0_6px_30px_rgba(0,0,0,0.35)]"
          : "border-b border-white/5 bg-gradient-to-b from-black/20 to-transparent",
      ].join(" ")}
      aria-label="Primary"
    >
      <div className="container-app flex h-14 items-center justify-between">
        {/* left: brand */}
        <Link href="/" className="group flex items-center gap-2">
          <span className="text-muted-foreground text-sm tracking-wide uppercase">Swearjar</span>
          <span className="text-gradient text-lg leading-none font-semibold">/</span>
          <span className="sr-only">Home</span>
        </Link>

        {/* center: links */}
        <ul className="hidden gap-6 md:flex">
          {links.map((l) => {
            const active = pathname === l.href
            return (
              <li key={l.href}>
                <Link
                  href={l.href}
                  className={[
                    "text-sm transition-colors",
                    active ? "text-foreground" : "text-muted-foreground hover:text-foreground",
                  ].join(" ")}
                >
                  {l.label}
                </Link>
              </li>
            )
          })}
        </ul>

        {/* right: GitHub + CTA */}
        <div className="flex items-center gap-3">
          <a
            href="https://github.com/your-org/your-repo"
            target="_blank"
            rel="noreferrer"
            aria-label="GitHub repository"
            className="text-muted-foreground hover:text-foreground inline-flex h-9 w-9 items-center justify-center rounded-md border border-white/10 transition-colors"
            title="GitHub"
          >
            <GitHubMark className="h-4 w-4" />
          </a>

          <Link
            href="/about#consent"
            className="btn-gradient h-9 px-3 text-sm"
            title="Opt-in / Opt-out"
          >
            Opt-in / Opt-out
          </Link>
        </div>
      </div>
    </nav>
  )
}
