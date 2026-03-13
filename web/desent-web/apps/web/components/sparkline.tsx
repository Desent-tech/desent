"use client"

type SparklineProps = {
  data: number[]
  max?: number
  color?: string
  height?: number
  unit?: string
}

const Y_TICKS = 3
const LEFT_MARGIN = 36

export function Sparkline({ data, max = 100, color = "var(--color-chart-1)", height = 60, unit = "" }: SparklineProps) {
  const effectiveMax = max > 0 ? max : 1

  // Y-axis tick values (bottom to top)
  const ticks = Array.from({ length: Y_TICKS }, (_, i) => Math.round((effectiveMax / (Y_TICKS - 1)) * i))

  if (data.length < 2) {
    return (
      <div className="flex" style={{ height }}>
        <div className="flex flex-col justify-between text-[10px] text-muted-foreground pr-2 py-[2px]" style={{ width: LEFT_MARGIN, flexShrink: 0 }}>
          {[...ticks].reverse().map((v, i) => (
            <span key={i} className="leading-none text-right">{v}{unit}</span>
          ))}
        </div>
        <svg className="flex-1" height={height} />
      </div>
    )
  }

  const padding = 2
  const h = height - padding * 2

  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * 100
    const y = padding + h - (Math.min(v, effectiveMax) / effectiveMax) * h
    return { x, y }
  })

  const linePath = points.map((p, i) => `${i === 0 ? "M" : "L"}${p.x},${p.y}`).join(" ")
  const areaPath = `${linePath} L${points[points.length - 1]!.x},${height} L${points[0]!.x},${height} Z`

  return (
    <div className="flex" style={{ height }}>
      <div className="flex flex-col justify-between text-[10px] text-muted-foreground pr-2 py-[2px]" style={{ width: LEFT_MARGIN, flexShrink: 0 }}>
        {[...ticks].reverse().map((v, i) => (
          <span key={i} className="leading-none text-right">{v}{unit}</span>
        ))}
      </div>
      <svg className="flex-1" viewBox={`0 0 100 ${height}`} preserveAspectRatio="none">
        <path d={areaPath} fill={color} opacity={0.15} />
        <path d={linePath} fill="none" stroke={color} strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
      </svg>
    </div>
  )
}
