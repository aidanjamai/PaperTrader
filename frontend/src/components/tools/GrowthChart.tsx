import React, { useMemo, useState } from 'react';
import { formatMoney } from '../primitives/format';

export interface GrowthSeriesPoint {
  year: number;
  balance: number;
  contributions: number;
}

interface GrowthChartProps {
  series: GrowthSeriesPoint[];
}

const VIEWBOX_WIDTH = 720;
const VIEWBOX_HEIGHT = 260;
const PADDING_TOP = 12;
const PADDING_RIGHT = 12;
const PADDING_BOTTOM = 28;
const PADDING_LEFT = 56;
const PLOT_WIDTH = VIEWBOX_WIDTH - PADDING_LEFT - PADDING_RIGHT;
const PLOT_HEIGHT = VIEWBOX_HEIGHT - PADDING_TOP - PADDING_BOTTOM;

const Y_TICKS = 4;

// Note: PriceChart has its own formatAxisMoney that keeps 2dp under $1k —
// stock prices and savings totals scale very differently, so the two formatters
// intentionally diverge.
const formatAxisMoney = (value: number): string => {
  if (value >= 1_000_000) return `$${(value / 1_000_000).toFixed(value >= 10_000_000 ? 0 : 1)}M`;
  if (value >= 1_000) return `$${(value / 1_000).toFixed(value >= 10_000 ? 0 : 1)}k`;
  return `$${value.toFixed(0)}`;
};

const formatYearLabel = (year: number): string => {
  if (Number.isInteger(year)) return `${year}y`;
  return `${year.toFixed(1)}y`;
};

const GrowthChart: React.FC<GrowthChartProps> = ({ series }) => {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const { yMax, xMax, balancePath, contribPath, areaPath, points, yTicks, xTicks } = useMemo(() => {
    const xMaxLocal = Math.max(...series.map((p) => p.year));
    const yMaxRaw = Math.max(...series.map((p) => p.balance));
    // Pad upward so the line doesn't touch the top edge.
    const yMaxLocal = yMaxRaw > 0 ? yMaxRaw * 1.05 : 1;

    const xScale = (year: number): number =>
      PADDING_LEFT + (xMaxLocal === 0 ? 0 : (year / xMaxLocal) * PLOT_WIDTH);
    const yScale = (value: number): number =>
      PADDING_TOP + PLOT_HEIGHT - (yMaxLocal === 0 ? 0 : (value / yMaxLocal) * PLOT_HEIGHT);

    const balancePts = series.map((p) => `${xScale(p.year).toFixed(2)},${yScale(p.balance).toFixed(2)}`);
    const contribPts = series.map((p) => `${xScale(p.year).toFixed(2)},${yScale(p.contributions).toFixed(2)}`);

    const balancePathLocal = `M${balancePts.join(' L')}`;
    const contribPathLocal = `M${contribPts.join(' L')}`;

    // Filled area between contributions (lower) and balance (upper) = compounded interest.
    const areaPathLocal = `M${balancePts.join(' L')} L${[...contribPts].reverse().join(' L')} Z`;

    const pointsLocal = series.map((p) => ({
      x: xScale(p.year),
      yBalance: yScale(p.balance),
      yContrib: yScale(p.contributions),
      data: p,
    }));

    const yTicksLocal = Array.from({ length: Y_TICKS + 1 }, (_, i) => {
      const value = (yMaxLocal / Y_TICKS) * i;
      return { value, y: yScale(value) };
    });

    // Show up to ~6 x-axis labels at integer years when possible.
    const wholeYears = Math.max(1, Math.floor(xMaxLocal));
    const step = Math.max(1, Math.ceil(wholeYears / 6));
    const xTicksLocal: { value: number; x: number }[] = [{ value: 0, x: xScale(0) }];
    for (let y = step; y <= wholeYears; y += step) {
      xTicksLocal.push({ value: y, x: xScale(y) });
    }
    if (xTicksLocal[xTicksLocal.length - 1].value !== xMaxLocal) {
      xTicksLocal.push({ value: xMaxLocal, x: xScale(xMaxLocal) });
    }

    return {
      yMax: yMaxLocal,
      xMax: xMaxLocal,
      balancePath: balancePathLocal,
      contribPath: contribPathLocal,
      areaPath: areaPathLocal,
      points: pointsLocal,
      yTicks: yTicksLocal,
      xTicks: xTicksLocal,
    };
  }, [series]);

  const handleMove = (event: React.MouseEvent<SVGSVGElement>) => {
    const svg = event.currentTarget;
    const rect = svg.getBoundingClientRect();
    const ratio = (event.clientX - rect.left) / rect.width;
    const x = ratio * VIEWBOX_WIDTH;
    let nearest = 0;
    let nearestDist = Infinity;
    points.forEach((p, idx) => {
      const dist = Math.abs(p.x - x);
      if (dist < nearestDist) {
        nearest = idx;
        nearestDist = dist;
      }
    });
    setHoverIndex(nearest);
  };

  const hovered = hoverIndex == null ? null : points[hoverIndex];

  return (
    <div>
      <div
        className="eyebrow"
        style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}
      >
        <span>Growth over time</span>
        <span className="mono" style={{ fontSize: 11, color: 'var(--ink-muted)', textTransform: 'none', letterSpacing: 0 }}>
          {hovered
            ? `${formatYearLabel(hovered.data.year)} · $${formatMoney(hovered.data.balance)}`
            : `${formatYearLabel(xMax)} horizon`}
        </span>
      </div>

      <svg
        viewBox={`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`}
        role="img"
        aria-label="Projected balance versus total contributions over time"
        style={{
          width: '100%',
          height: 'auto',
          display: 'block',
          fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
        }}
        onMouseMove={handleMove}
        onMouseLeave={() => setHoverIndex(null)}
      >
        {/* Y gridlines + axis labels */}
        {yTicks.map((tick, idx) => (
          <g key={`y-${idx}`}>
            <line
              x1={PADDING_LEFT}
              x2={PADDING_LEFT + PLOT_WIDTH}
              y1={tick.y}
              y2={tick.y}
              stroke="var(--hairline)"
              strokeWidth={1}
              strokeDasharray={idx === 0 ? undefined : '2 4'}
            />
            <text
              x={PADDING_LEFT - 8}
              y={tick.y + 3}
              fill="var(--ink-muted)"
              fontSize={10}
              textAnchor="end"
            >
              {formatAxisMoney(tick.value)}
            </text>
          </g>
        ))}

        {/* X axis labels */}
        {xTicks.map((tick, idx) => (
          <text
            key={`x-${idx}`}
            x={tick.x}
            y={PADDING_TOP + PLOT_HEIGHT + 16}
            fill="var(--ink-muted)"
            fontSize={10}
            textAnchor="middle"
          >
            {formatYearLabel(tick.value)}
          </text>
        ))}

        {/* Interest area (between contributions and balance) */}
        <path d={areaPath} fill="var(--gain-tint)" stroke="none" />

        {/* Contributions line */}
        <path
          d={contribPath}
          fill="none"
          stroke="var(--ink-muted)"
          strokeWidth={1.25}
          strokeDasharray="4 3"
        />

        {/* Balance line */}
        <path d={balancePath} fill="none" stroke="var(--accent)" strokeWidth={2} />

        {/* Hover marker */}
        {hovered && (
          <g>
            <line
              x1={hovered.x}
              x2={hovered.x}
              y1={PADDING_TOP}
              y2={PADDING_TOP + PLOT_HEIGHT}
              stroke="var(--ink-muted)"
              strokeWidth={1}
              strokeDasharray="2 3"
            />
            <circle cx={hovered.x} cy={hovered.yBalance} r={3.5} fill="var(--accent)" />
            <circle cx={hovered.x} cy={hovered.yContrib} r={2.5} fill="var(--ink-muted)" />
          </g>
        )}
      </svg>

      <div
        style={{
          marginTop: 10,
          display: 'flex',
          gap: 18,
          flexWrap: 'wrap',
          fontSize: 12,
          color: 'var(--ink-muted)',
        }}
      >
        <LegendSwatch color="var(--accent)" label="Balance" />
        <LegendSwatch color="var(--ink-muted)" label="Contributions" dashed />
        <LegendSwatch color="var(--gain-tint)" label="Compounded interest" filled />
        {hovered && (
          <span className="mono" style={{ marginLeft: 'auto' }}>
            contrib ${formatMoney(hovered.data.contributions)}
          </span>
        )}
      </div>

      {/* Quiet sr-only summary */}
      <span style={{ position: 'absolute', width: 1, height: 1, overflow: 'hidden', clip: 'rect(0 0 0 0)' }}>
        Projection over {xMax.toFixed(1)} years reaching ${formatMoney(yMax)}.
      </span>
    </div>
  );
};

const LegendSwatch: React.FC<{ color: string; label: string; dashed?: boolean; filled?: boolean }> = ({
  color,
  label,
  dashed,
  filled,
}) => (
  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
    <span
      aria-hidden="true"
      style={
        filled
          ? { width: 14, height: 8, background: color, border: '1px solid var(--hairline)', display: 'inline-block' }
          : {
              width: 14,
              height: 0,
              borderTop: `2px ${dashed ? 'dashed' : 'solid'} ${color}`,
              display: 'inline-block',
            }
      }
    />
    {label}
  </span>
);

export default GrowthChart;
