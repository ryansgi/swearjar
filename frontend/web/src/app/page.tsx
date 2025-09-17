import Hero from "@/components/Hero.server"
import ExploreCalendar from "@/components/explore/ExploreCalendar.client"
import BirdsEye from "@/components/explore/BirdsEye.client"

export default async function Dashboard() {
  // In the future, fetch real stats on the server here
  const repos = 128,
    actors = 642,
    hits = 4213,
    cents = 25

  return (
    <main className="bg-aurora relative min-h-dvh overflow-hidden">
      <div className="bg-grid absolute inset-0" />
      <Hero repos={repos} actors={actors} hits={hits} jarCentsPerCuss={cents} />

      <section className="container-app relative space-y-6 py-6">
        <BirdsEye initialYear={2025} initialVariant="commit-crimes" />
      </section>
    </main>
  )
}
