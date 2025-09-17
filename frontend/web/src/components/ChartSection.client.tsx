"use client"

import { Card } from "@/components/ui/card"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart"
import { Bar, BarChart, XAxis, YAxis } from "recharts"

const data = [
  { hour: "10:00", hits: 120 },
  { hour: "11:00", hits: 340 },
  { hour: "12:00", hits: 210 },
]

export default function ChartSection() {
  return (
    <div className="card-surface chart-theme p-4">
      <h2 className="text-lg font-medium">Hits by hour</h2>
      <Tabs defaultValue="summary" className="max-w-5xl">
        <TabsList>
          <TabsTrigger value="summary">Summary</TabsTrigger>
          <TabsTrigger value="details">Details</TabsTrigger>
        </TabsList>
        <TabsContent value="summary" className="mt-4">
          <Card className="mt-0 p-4">
            <ChartContainer
              className="mt-2 h-72"
              config={{ hits: { label: "Hits", color: "hsl(var(--primary))" } }}
            >
              <BarChart data={data}>
                <XAxis dataKey="hour" />
                <YAxis />
                <Bar dataKey="hits" radius={4} />
                <ChartTooltip content={<ChartTooltipContent />} />
              </BarChart>
            </ChartContainer>
          </Card>
        </TabsContent>
        <TabsContent value="details" className="mt-4">
          <Card className="p-4">More cards/tables go here…</Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
