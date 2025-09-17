type Props = { repos: number; actors: number; hits: number; jarCentsPerCuss: number }

const TAGLINES = [
  "Commit early, curse often.",
  "Swears happen. We graph them.",
  "Profanity, but make it charts.",
  "Beep-boop, counting naughty words.",
  "'Fuck you, Dependabot.' - someone, somewhere",
  "Stats only THAT ONE DEV would ever care about...",
]

export default function Hero({ repos, actors, hits, jarCentsPerCuss }: Props) {
  const tagline = TAGLINES[Math.floor(Math.random() * TAGLINES.length)] // server-only
  const nf = new Intl.NumberFormat("en-US")
  const cf = new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
  })
  const todaysJar = cf.format((hits * jarCentsPerCuss) / 100)

  return (
    <header className="sticky top-0 z-20">
      <div className="container-app">
        <div className="hero-shell card-surface overflow-hidden px-5 py-7 md:px-8 md:py-9">
          <div className="grid gap-8 md:grid-cols-[1fr,420px] md:items-end">
            <div>
              <p className="text-muted-foreground text-sm tracking-wide">
                Expletive-Driven Development, visualized.
              </p>
              <h1 className="text-3xl leading-tight font-semibold text-balance md:text-5xl">
                A playful look at <span className="text-gradient">profanity</span> in public GitHub
                text
              </h1>
              <p className="text-muted-foreground mt-3 max-w-prose text-base text-pretty md:text-lg">
                Swearjar tracks cusses across commits, issues, pull requests, and comments—purely
                for fun. No shaming, just vibes, trends, and maybe a few coins in the jar.
              </p>
              <p className="text-muted-foreground/90 mt-2 text-sm italic">{tagline}</p>
            </div>

            <aside className="space-y-4">
              <div className="px-4 py-4">
                <div className="text-muted-foreground text-xs tracking-wide uppercase">
                  Today's swearjar (at {jarCentsPerCuss}¢/expletive)
                </div>
                <div className="mt-1 text-4xl font-semibold tabular-nums md:text-5xl">
                  {todaysJar}
                </div>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <Stat
                  label="repositories"
                  value={nf.format(repos)}
                  accent="from-[--color-chart-4] to-[--color-chart-1]"
                />
                <Stat
                  label="actors"
                  value={nf.format(actors)}
                  accent="from-[--color-chart-2] to-[--color-chart-5]"
                />
                <Stat
                  label="total cusses"
                  value={nf.format(hits)}
                  accent="from-[--color-chart-1] to-[--color-chart-2]"
                />
              </div>
            </aside>
          </div>
        </div>
      </div>
    </header>
  )
}

function Stat({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <div className="group bg-muted/30 relative overflow-hidden rounded-lg border px-3 py-2 text-center backdrop-blur">
      <div
        className={`pointer-events-none absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r ${accent}`}
      />
      <div className="text-muted-foreground text-xs tracking-wide uppercase">{label}</div>
      <div className="mt-1 text-xl font-semibold tabular-nums">{value}</div>
    </div>
  )
}
