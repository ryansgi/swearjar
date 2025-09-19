"use client"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

export function LeaderboardSkeleton({
  title,
  withTrend = false,
}: {
  title: string
  withTrend?: boolean
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-12 gap-3">
          <div className="text-muted-foreground col-span-8 text-sm">Name</div>
          <div className="text-muted-foreground col-span-2 text-right text-sm">Score</div>
          <div className="text-muted-foreground col-span-2 text-right text-sm">
            {withTrend ? "Trend" : "Rank"}
          </div>
        </div>
        <div className="mt-2 space-y-2">
          {Array.from({ length: 10 }).map((_, i) => (
            <div key={i} className="grid grid-cols-12 items-center gap-3">
              <div className="col-span-8 flex items-center gap-2">
                <Skeleton className="h-6 w-6 rounded-full" />
                <Skeleton className="h-4 w-40" />
              </div>
              <div className="col-span-2">
                <div className="flex justify-end">
                  <Skeleton className="h-4 w-12" />
                </div>
              </div>
              <div className="col-span-2">
                <div className="flex justify-end">
                  <Skeleton className="h-4 w-16" />
                </div>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
