import { useEffect, useState } from 'react'
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import type { DayActivity } from '#/lib/use-admin-overview'

interface ActivityChartProps {
  data: DayActivity[]
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00')
  return d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' })
}

export function AdminActivityChart({ data }: ActivityChartProps) {
  const [mounted, setMounted] = useState(false)
  useEffect(() => {
    setMounted(true)
  }, [])

  const chartData = data.map((d) => ({
    ...d,
    label: formatDate(d.date),
  }))

  if (!mounted) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium">Room activity</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-64" />
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">Room activity</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={chartData} margin={{ top: 5, right: 10, left: -10, bottom: 0 }}>
              <defs>
                <linearGradient id="roomsGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="activeRoomsGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="hsl(var(--chart-2))" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="hsl(var(--chart-2))" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="participantsGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="hsl(var(--accent))" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="hsl(var(--accent))" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" className="stroke-border" vertical={false} />
              <XAxis
                dataKey="label"
                tick={{ fontSize: 11 }}
                className="text-muted-foreground"
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fontSize: 11 }}
                className="text-muted-foreground"
                tickLine={false}
                axisLine={false}
                allowDecimals={false}
              />
              <Tooltip
                contentStyle={{
                  borderRadius: 0,
                  border: '1px solid hsl(var(--border))',
                  fontSize: 12,
                }}
              />
              <Area
                type="monotone"
                dataKey="roomsCreated"
                stroke="hsl(var(--primary))"
                fill="url(#roomsGradient)"
                strokeWidth={2}
                name="Rooms created"
              />
              <Area
                type="monotone"
                dataKey="roomsActive"
                stroke="hsl(var(--chart-2))"
                fill="url(#activeRoomsGradient)"
                strokeWidth={2}
                name="Active rooms"
              />
              <Area
                type="monotone"
                dataKey="participants"
                stroke="hsl(var(--accent))"
                fill="url(#participantsGradient)"
                strokeWidth={2}
                name="Participants"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  )
}
